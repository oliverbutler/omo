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
	"math"
	"mime/multipart"
	"net/http"
	"oliverbutler/lib/database"
	"oliverbutler/lib/logging"
	"oliverbutler/lib/storage"
	"oliverbutler/lib/tracing"
	"strings"
	"sync"
	"time"

	"github.com/buckket/go-blurhash"
	"github.com/disintegration/imaging"
	"github.com/lucsky/cuid"
	"github.com/rwcarlsen/goexif/exif"
)

type PhotoService struct {
	storage *storage.StorageService
	db      *database.DatabaseService
}

func NewPhotoService(storage *storage.StorageService, db *database.DatabaseService) *PhotoService {
	return &PhotoService{
		storage: storage,
		db:      db,
	}
}

type Photo struct {
	ID              string
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
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (s *PhotoService) UploadPhotos(ctx context.Context, r *http.Request) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.UploadPhotos")
	defer span.End()

	slog.Info("Starting upload photos")

	err := r.ParseMultipartForm(100 << 20) // 100 MB limit
	if err != nil {
		return fmt.Errorf("failed to parse multipart form: %w", err)
	}

	files := r.MultipartForm.File["photo"]
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))

	for _, fileHeader := range files {
		wg.Add(1)
		go func(fh *multipart.FileHeader) {
			defer wg.Done()

			// Store the original image
			storeImageResult, err := s.storeOriginalImage(ctx, fh)
			if err != nil {
				errChan <- fmt.Errorf("failed to store original image: %w", err)
				return
			}

			// Process the image
			err = s.processPhoto(ctx, storeImageResult.ID, storeImageResult.Name)
			if err != nil {
				errChan <- fmt.Errorf("failed to process photo %s: %w", storeImageResult.ID, err)
				return
			}
		}(fileHeader)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *PhotoService) processPhoto(ctx context.Context, photoId string, imageName string) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.processPhoto")
	defer span.End()

	logger := slog.With("photoId", photoId)
	logger.Info("Processing photo")

	// Generate previews concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	sizes := []struct {
		name  string
		width int
	}{
		{"small", 300},
		{"medium", 768},
		{"large", 1920},
	}

	for _, size := range sizes {
		wg.Add(1)
		go func(sizeName string, width int) {
			defer wg.Done()
			err := s.generatePreview(ctx, photoId, imageName, sizeName, width)
			if err != nil {
				errChan <- fmt.Errorf("failed to generate %s preview: %w", sizeName, err)
			}
		}(size.name, size.width)
	}

	// Wait for preview generation to complete
	wg.Wait()
	close(errChan)

	// Check for preview generation errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// Generate metadata and blurhash
	metadata, err := s.generateBlurHashAndMetadata(ctx, photoId, imageName)
	if err != nil {
		return fmt.Errorf("failed to generate metadata: %w", err)
	}

	// Write to database
	err = s.insertPhoto(ctx, &Photo{
		ID:              photoId,
		Name:            imageName,
		BlurHash:        metadata.BlurHash,
		Width:           metadata.Width,
		Height:          metadata.Height,
		Lens:            metadata.Lens,
		Aperature:       metadata.Aperature,
		ShutterSpeed:    metadata.ShutterSpeed,
		ISO:             metadata.ISO,
		FocalLength:     metadata.FocalLength,
		FocalLength35mm: metadata.FocalLength35mm,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to write to database: %w", err)
	}

	logger.Info("Photo processed successfully")
	return nil
}

func (s *PhotoService) generatePreview(ctx context.Context, photoId string, filename string, sizeName string, width int) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.generatePreview")
	defer span.End()

	logger := slog.With("photoId", photoId, "size", sizeName, "width", width)
	logger.Info("Generating preview")

	originalPhoto, err := s.storage.StorageRepo.GetItem(ctx, "photos", photoId, filename)
	if err != nil {
		return fmt.Errorf("failed to get original photo: %w", err)
	}

	originalPhotoContent, err := s.storage.StorageRepo.GetItemContent(ctx, originalPhoto)
	if err != nil {
		return fmt.Errorf("failed to get original photo content: %w", err)
	}
	defer originalPhotoContent.Close()

	originalImage, _, err := image.Decode(originalPhotoContent)
	if err != nil {
		return fmt.Errorf("failed to decode original image: %w", err)
	}

	preview, err := s.generateResizedImage(originalImage, width)
	if err != nil {
		return fmt.Errorf("failed to generate resized image: %w", err)
	}

	_, err = s.storage.StorageRepo.PutItem(ctx, "photos", photoId, sizeName+".jpg", preview, int64(preview.Len()), "image/jpeg")
	if err != nil {
		return fmt.Errorf("failed to store preview image: %w", err)
	}

	return nil
}

type GeneratePreviewActivityParams struct {
	PhotoId  string
	SizeName string
	Width    int
	Filename string
}

func (s *PhotoService) GeneratePreviewActivity(ctx context.Context, params GeneratePreviewActivityParams) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GeneratePreviewActivity")
	defer span.End()

	logging.OmoLogger.Info("Generating preview for photo", "photoId", params.PhotoId, "size", params.SizeName, "width", params.Width)

	/// Pull down original photo from storage
	originalPhoto, err := s.storage.StorageRepo.GetItem(ctx, "photos", params.PhotoId, params.Filename)
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

	logging.OmoLogger.Info("Preview generated successfully", "photoId", params.PhotoId, "size", params.SizeName, "width", params.Width)

	return nil
}

type GenerateBlurHashAndMetadataActivityParams struct {
	PhotoId  string
	Filename string
}

func (s *PhotoService) generateBlurHashAndMetadata(ctx context.Context, photoId string, filename string) (*PhotoMetaData, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GenerateBlurHashAndMetadataActivity")
	defer span.End()

	originalPhoto, err := s.storage.StorageRepo.GetItem(ctx, "photos", photoId, filename)
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
			num, den, _ := val.Rat2(0)
			if den != 0 {
				focalLength = fmt.Sprintf("%dmm", int(float64(num)/float64(den)))
			}
		}
		if val, err := ex.Get(exif.FocalLengthIn35mmFilm); err == nil {
			mm, _ := val.Int(0)
			focalLength35mm = fmt.Sprintf("%dmm", mm)
		}
		if val, err := ex.Get(exif.LensModel); err == nil {
			lens = strings.Trim(val.String(), "\"") // Remove quotes

			// Clean up common lens model strings
			lens = strings.TrimSpace(lens)

			// Remove common technical codes (like B061)
			if idx := strings.LastIndex(lens, " B"); idx > 0 {
				lens = strings.TrimSpace(lens[:idx])
			}

			// Try to get lens make if available
			if lensMake, err := ex.Get(exif.LensMake); err == nil {
				make := strings.Trim(lensMake.String(), "\"")
				if !strings.Contains(strings.ToLower(lens), strings.ToLower(make)) {
					lens = make + " " + lens
				}
			}
		}
		if val, err := ex.Get(exif.ApertureValue); err == nil {
			num, den, _ := val.Rat2(0)
			if den != 0 {
				aperature = fmt.Sprintf("%.1f", math.Pow(2, float64(num)/float64(den)/2))
			}
		}
		if val, err := ex.Get(exif.ShutterSpeedValue); err == nil {
			num, den, _ := val.Rat2(0)
			if den != 0 {
				speed := math.Pow(2, -float64(num)/float64(den)) // Note the negative exponent here
				if speed >= 1 {
					if speed == float64(int64(speed)) {
						shutterSpeed = fmt.Sprintf("%ds", int64(speed))
					} else {
						shutterSpeed = fmt.Sprintf("%.1fs", speed)
					}
				} else {
					shutterSpeed = fmt.Sprintf("1/%d", int(1/speed+0.5)) // Round to nearest integer
				}
			}
		}
		if val, err := ex.Get(exif.ISOSpeedRatings); err == nil {
			isoVal, _ := val.Int(0)
			iso = fmt.Sprintf("%d", isoVal)
		}
	} else {
		slog.Info("No EXIF data available for image", "photoId", photoId, "error", err)
	}

	return &PhotoMetaData{
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

type StoreOriginalImageResult struct {
	ID   string
	Name string
}

func (s *PhotoService) storeOriginalImage(ctx context.Context, fileHeader *multipart.FileHeader) (*StoreOriginalImageResult, error) {
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

	fileName := fileHeader.Filename

	// Store the original file
	_, err = s.storage.StorageRepo.PutItem(ctx, "photos", id, fileName, bytes.NewReader(fileBytes), int64(len(fileBytes)), fileHeader.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to store original file: %w", err)
	}

	return &StoreOriginalImageResult{
		ID:   id,
		Name: fileName,
	}, nil
}

func (s *PhotoService) GetPhoto(ctx context.Context, id string) (*Photo, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GetPhoto")
	defer span.End()
	return s.getPhoto(ctx, id)
}

func (s *PhotoService) GetPhotoBuffer(ctx context.Context, id string, quality string) (io.ReadCloser, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "PhotoService.GetPhotoBuffer")
	defer span.End()

	photo, err := s.getPhoto(ctx, id)

	filename := quality + ".jpg"

	if quality == "original" {
		filename = photo.Name
	}

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
		INSERT INTO photos (
			id, name, blur_hash, width, height,
			lens, aperature, shutter_speed, iso,
			focal_length, focal_length_35mm,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := s.db.Pool.Exec(ctx, query,
		photo.ID, photo.Name, photo.BlurHash, photo.Width, photo.Height,
		photo.Lens, photo.Aperature, photo.ShutterSpeed, photo.ISO,
		photo.FocalLength, photo.FocalLength35mm,
		photo.CreatedAt, photo.UpdatedAt,
	)
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
    SELECT 
      id, name, blur_hash, width, height,
      lens, aperature, shutter_speed, iso,
      focal_length, focal_length_35mm,
      created_at, updated_at
    FROM photos
    WHERE id = $1
  `

	var photo Photo
	err := s.db.Pool.QueryRow(ctx, query, id).Scan(
		&photo.ID, &photo.Name, &photo.BlurHash, &photo.Width, &photo.Height,
		&photo.Lens, &photo.Aperature, &photo.ShutterSpeed, &photo.ISO,
		&photo.FocalLength, &photo.FocalLength35mm,
		&photo.CreatedAt, &photo.UpdatedAt,
	)

	return &photo, err
}
