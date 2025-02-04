package photos

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"oliverbutler/lib/database"
	"oliverbutler/lib/storage"
	"oliverbutler/lib/tracing"
	"oliverbutler/lib/workflow"
	"strings"
	"time"

	"github.com/buckket/go-blurhash"
	"github.com/disintegration/imaging"
	"github.com/lucsky/cuid"
	"github.com/rwcarlsen/goexif/exif"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	temporalWorkflow "go.temporal.io/sdk/workflow"
)

type PhotoService struct {
	storage  *storage.StorageService
	db       *database.DatabaseService
	workflow *workflow.WorkflowService
}

func NewPhotoService(storage *storage.StorageService, db *database.DatabaseService, workflow *workflow.WorkflowService) *PhotoService {
	service := &PhotoService{
		storage:  storage,
		db:       db,
		workflow: workflow,
	}

	workflow.RegisterWorkflow("PhotoUpload", service.NewPhotoUploadWorkflow())
	workflow.RegisterActivity(service.GeneratePreviewActivity)
	workflow.RegisterActivity(service.GenerateBlurHashAndMetadataActivity)
	workflow.RegisterActivity(service.WritePhotoToDBActivity)

	return service
}

type Photo struct {
	ID        string
	Name      string
	BlurHash  string
	Width     int
	Height    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *PhotoService) UploadPhotosAndStartWorkflows(ctx context.Context, r *http.Request) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.UploadPhotosAndStartWorkflows")
	defer span.End()

	slog.Info("Starting upload photos")

	err := r.ParseMultipartForm(100 << 20) // 100 MB limit
	if err != nil {
		return fmt.Errorf("failed to parse multipart form: %w", err)
	}

	files := r.MultipartForm.File["photo"]

	for _, fileHeader := range files {

		photoId, err := s.storeOriginalImage(ctx, fileHeader)
		if err != nil {
			return fmt.Errorf("failed to store original image: %w", err)
		}

		s.workflow.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
			ID:        "photo_upload_" + *photoId,
			TaskQueue: "oliverbutler",
		}, "PhotoUpload", *photoId)
	}

	return nil
}

func (s *PhotoService) NewPhotoUploadWorkflow() func(ctx temporalWorkflow.Context, photoId string) (string, error) {
	return func(ctx temporalWorkflow.Context, photoId string) (string, error) {
		ao := temporalWorkflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 6,
			},
		}
		ctx = temporalWorkflow.WithActivityOptions(ctx, ao)

		logger := temporalWorkflow.GetLogger(ctx)
		logger.Info("Starting photo upload workflow", "photoId", photoId)

		var futureSmall, futureMedium, futureLarge temporalWorkflow.Future

		futureSmall = temporalWorkflow.ExecuteActivity(ctx, s.GeneratePreviewActivity, GeneratePreviewActivityParams{
			PhotoId: photoId, SizeName: "small", Width: 300,
		})

		futureMedium = temporalWorkflow.ExecuteActivity(ctx, s.GeneratePreviewActivity, GeneratePreviewActivityParams{
			PhotoId: photoId, SizeName: "medium", Width: 768,
		})

		futureLarge = temporalWorkflow.ExecuteActivity(ctx, s.GeneratePreviewActivity, GeneratePreviewActivityParams{
			PhotoId: photoId, SizeName: "large", Width: 1920,
		})

		// Wait for all activities to complete and check for errors
		err := futureSmall.Get(ctx, nil)
		err = futureMedium.Get(ctx, nil)
		err = futureLarge.Get(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("failed to generate preview: %w", err)
		}

		var photoMetaData PhotoMetaData
		err = temporalWorkflow.ExecuteActivity(ctx, s.GenerateBlurHashAndMetadataActivity, photoId).Get(ctx, &photoMetaData)

		// log out metadata
		logger.Info("Photo metadata", "photoId", photoId, "metadata", photoMetaData)

		// Now that we have the metadata, we can write the photo to the database
		err = temporalWorkflow.ExecuteActivity(ctx, s.WritePhotoToDBActivity, Photo{
			ID:        photoId,
			Name:      photoMetaData.Name,
			BlurHash:  photoMetaData.BlurHash,
			Width:     photoMetaData.Width,
			Height:    photoMetaData.Height,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}).Get(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("failed to write photo to DB: %w", err)
		}

		return "Successfully processed image: " + photoId, nil
	}
}

type GeneratePreviewActivityParams struct {
	PhotoId  string
	SizeName string
	Width    int
}

func (s *PhotoService) GeneratePreviewActivity(ctx context.Context, params GeneratePreviewActivityParams) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GeneratePreviewActivity")
	defer span.End()

	logger := activity.GetLogger(ctx)

	logger.Info("Generating preview for photo", "photoId", params.PhotoId, "size", params.SizeName, "width", params.Width)

	/// Pull down original photo from storage
	originalPhoto, err := s.storage.StorageRepo.GetItem(ctx, "photos", params.PhotoId, "original.jpg")
	if err != nil {
		return fmt.Errorf("failed to get original photo: %w", err)
	}

	/// Decode original photo
	originalPhotoContent, err := s.storage.StorageRepo.GetItemContent(ctx, originalPhoto)
	if err != nil {
		return fmt.Errorf("failed to get original photo content: %w", err)
	}

	originalImage, _, err := image.Decode(originalPhotoContent)
	if err != nil {
		return fmt.Errorf("failed to decode original image: %w", err)
	}

	/// Generate preview image
	preview, err := s.generateResizedImage(originalImage, params.Width)
	if err != nil {
		return fmt.Errorf("failed to generate resized image: %w", err)
	}

	/// Store preview image in storage
	_, err = s.storage.StorageRepo.PutItem(ctx, "photos", params.PhotoId, params.SizeName+".jpg", preview, int64(preview.Len()), "image/jpeg")
	if err != nil {
		return fmt.Errorf("failed to store preview image: %w", err)
	}

	logger.Info("Preview generated successfully", "photoId", params.PhotoId, "size", params.SizeName, "width", params.Width)

	return nil
}

func (s *PhotoService) GenerateBlurHashAndMetadataActivity(ctx context.Context, photoId string) (*PhotoMetaData, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GenerateBlurHashAndMetadataActivity")
	defer span.End()

	originalPhoto, err := s.storage.StorageRepo.GetItem(ctx, "photos", photoId, "original.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to get original photo: %w", err)
	}

	originalPhotoContent, err := s.storage.StorageRepo.GetItemContent(ctx, originalPhoto)
	if err != nil {
		return nil, fmt.Errorf("failed to get original photo content: %w", err)
	}
	defer originalPhotoContent.Close()

	// Read the content into a byte slice
	contentBytes, err := io.ReadAll(originalPhotoContent)
	if err != nil {
		return nil, fmt.Errorf("failed to read photo content: %w", err)
	}

	originalImage, _, err := image.Decode(bytes.NewReader(contentBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode original image: %w", err)
	}

	start := time.Now()
	defer func() {
		slog.Info("BlurHash generation time", "duration", time.Since(start))
	}()

	tinyImageForBlurHash, err := s.generateResizedImage(originalImage, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tiny image for BlurHash: %w", err)
	}

	tinyImage, _, err := image.Decode(bytes.NewReader(tinyImageForBlurHash.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("failed to decode original image: %w", err)
	}

	hash, err := blurhash.Encode(4, 3, tinyImage)
	if err != nil {
		return nil, fmt.Errorf("failed to generate BlurHash: %w", err)
	}

	// Initialize empty EXIF values
	var focalLength, focalLength35mm, lens, aperature, shutterSpeed, iso string

	// Try to read EXIF data, but continue if it's not available
	if ex, err := exif.Decode(bytes.NewReader(contentBytes)); err == nil {
		// Get EXIF values, defaulting to empty string if not available
		if val, err := ex.Get(exif.FocalLength); err == nil {
			focalLength = val.String()
		}
		if val, err := ex.Get(exif.FocalLengthIn35mmFilm); err == nil {
			focalLength35mm = val.String()
		}
		if val, err := ex.Get(exif.LensModel); err == nil {
			lens = val.String()
		}
		if val, err := ex.Get(exif.ApertureValue); err == nil {
			aperature = val.String()
		}
		if val, err := ex.Get(exif.ShutterSpeedValue); err == nil {
			shutterSpeed = val.String()
		}
		if val, err := ex.Get(exif.ISOSpeedRatings); err == nil {
			iso = val.String()
		}
	} else {
		slog.Info("No EXIF data available for image", "photoId", photoId, "error", err)
	}

	return &PhotoMetaData{
		Name:            originalPhoto.Name,
		BlurHash:        hash,
		Width:           originalImage.Bounds().Dx(),
		Height:          originalImage.Bounds().Dy(),
		FocalLength:     focalLength,
		FocalLength35mm: focalLength35mm,
		Lens:            lens,
		Aperature:       aperature,
		ShutterSpeed:    shutterSpeed,
		ISO:             iso,
	}, nil
}

type PhotoMetaData struct {
	Name            string
	BlurHash        string
	Width           int
	Height          int
	Lens            string
	Aperature       string
	ShutterSpeed    string
	ISO             string
	FocalLength     string
	FocalLength35mm string
}

func (s *PhotoService) WritePhotoToDBActivity(ctx context.Context, photo Photo) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.WritePhotoToDBActivity")
	defer span.End()

	err := s.insertPhoto(ctx, &photo)
	if err != nil {
		return fmt.Errorf("failed to insert photo into database: %w", err)
	}

	return nil
}

func (s *PhotoService) storeOriginalImage(ctx context.Context, fileHeader *multipart.FileHeader) (*string, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.storeOriginalImage")
	defer span.End()

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	id := cuid.New()
	slog.Info("Processing photo", "id", id, "filename", fileHeader.Filename)

	ext := strings.ToLower(fileHeader.Filename[strings.LastIndex(fileHeader.Filename, "."):])

	// Store the original file
	_, err = s.storage.StorageRepo.PutItem(ctx, "photos", id, "original"+ext, bytes.NewReader(fileBytes), int64(len(fileBytes)), fileHeader.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to store original file: %w", err)
	}

	return &id, nil
}

func (s *PhotoService) GetPhoto(ctx context.Context, id string) (*Photo, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GetPhoto")
	defer span.End()
	return s.getPhoto(ctx, id)
}

func (s *PhotoService) GetPhotoBuffer(ctx context.Context, id string, quality string) (io.ReadCloser, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GetPhotoBuffer")
	defer span.End()

	filename := quality + ".jpg"
	item, err := s.storage.StorageRepo.GetItem(ctx, "photos", id, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s photo: %w", quality, err)
	}

	return s.storage.StorageRepo.GetItemContent(ctx, item)
}

func (s *PhotoService) DeletePhoto(ctx context.Context, id string) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.DeletePhoto")
	defer span.End()

	slog.Info("Deleting photo", "id", id)

	// Get all photos
	photos, err := s.getAllPhotos(ctx)
	if err != nil {
		return fmt.Errorf("failed to get photos: %w", err)
	}

	// Check if photo exists
	var photoExists bool
	for _, photo := range photos {
		if photo.ID == id {
			photoExists = true
			break
		}
	}
	if !photoExists {
		return fmt.Errorf("photo with id %s does not exist", id)
	}

	// Delete folder
	slog.Info("Deleting folder from storage")
	err = s.storage.StorageRepo.DeleteFolder(ctx, "photos", id)
	if err != nil {
		return fmt.Errorf("failed to delete folder from storage: %w", err)
	}

	// Delete metadata from database
	slog.Info("Deleting metadata from database")
	query := `
    DELETE FROM photos
    WHERE id = $1
  `
	_, err = s.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete photo metadata from database: %w", err)
	}

	slog.Info("Photo deleted successfully", "id", id)
	return nil
}

func (s *PhotoService) generateResizedImage(src image.Image, width int) (*bytes.Buffer, error) {
	_, span := tracing.OmoTracer.Start(context.Background(), "PhotoService.generateResizedImage")
	defer span.End()

	start := time.Now()
	defer func() {
		slog.Info("Image resizing time", "width", width, "duration", time.Since(start))
	}()

	// Resize image
	resized := imaging.Resize(src, width, 0, imaging.Lanczos)

	// Encode image
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, imaging.JPEG); err != nil {
		return nil, fmt.Errorf("failed to encode resized image: %w", err)
	}

	return &buf, nil
}

func (s *PhotoService) GetPhotos(ctx context.Context) ([]Photo, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GetPhotos")
	defer span.End()

	slog.Info("Fetching photos from database")
	photos, err := s.getAllPhotos(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos from database: %w", err)
	}
	return photos, nil
}

func (s *PhotoService) insertPhoto(ctx context.Context, photo *Photo) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.insertPhoto")
	defer span.End()

	slog.Info("Inserting photo into database", "id", photo.ID)
	query := `
		INSERT INTO photos (id, name, blur_hash, width, height, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.Pool.Exec(ctx, query, photo.ID, photo.Name, photo.BlurHash, photo.Width, photo.Height, photo.CreatedAt, photo.UpdatedAt)
	return err
}

func (s *PhotoService) getAllPhotos(ctx context.Context) ([]Photo, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.getAllPhotos")
	defer span.End()

	query := `
		SELECT id, name, blur_hash, width, height, created_at, updated_at
		FROM photos
		ORDER BY created_at DESC
	`
	rows, err := s.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []Photo
	for rows.Next() {
		var photo Photo
		if err := rows.Scan(&photo.ID, &photo.Name, &photo.BlurHash, &photo.Width, &photo.Height, &photo.CreatedAt, &photo.UpdatedAt); err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}

	return photos, rows.Err()
}

func (s *PhotoService) getPhoto(ctx context.Context, id string) (*Photo, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.getPhoto")
	defer span.End()

	query := `
    SELECT id, name, blur_hash, width, height, created_at, updated_at
    FROM photos
    WHERE id = $1
  `

	var photo Photo
	err := s.db.Pool.QueryRow(ctx, query, id).Scan(&photo.ID, &photo.Name, &photo.BlurHash, &photo.Width, &photo.Height, &photo.CreatedAt, &photo.UpdatedAt)

	return &photo, err
}
