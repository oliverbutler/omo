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
	"net/http"
	"oliverbutler/lib/database"
	"oliverbutler/lib/storage"
	"strings"
	"sync"
	"time"

	"github.com/buckket/go-blurhash"
	"github.com/disintegration/imaging"
	"github.com/lucsky/cuid"
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
	ID        string
	Name      string
	BlurHash  string
	Width     int
	Height    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *PhotoService) UploadPhoto(ctx context.Context, r *http.Request) (*Photo, error) {
	slog.Info("Starting upload photo")

	// Parse the multipart form data
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		return nil, fmt.Errorf("failed to parse multipart form: %w", err)
	}

	// Get the file from the form data
	file, header, err := r.FormFile("photo")
	if err != nil {
		return nil, fmt.Errorf("failed to get file from form: %w", err)
	}
	defer file.Close()

	// Read file content into buffer
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Generate a unique ID for the photo
	id := cuid.New()
	slog.Info("Generated unique ID for photo", "id", id)

	// Generate file paths for original, large, medium, and small versions
	ext := strings.ToLower(header.Filename[strings.LastIndex(header.Filename, "."):])

	// Store the original file
	slog.Info("Storing original file")
	_, err = s.storage.StorageRepo.PutItem(ctx, "photos", id, "original"+ext, bytes.NewReader(fileBytes), int64(len(fileBytes)), header.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to store original file: %w", err)
	}

	// Generate previews and save them to MinIO in parallel
	slog.Info("Generating and storing previews")
	if err := s.generateAndStorePreviews(ctx, fileBytes, id); err != nil {
		return nil, fmt.Errorf("failed to generate and store previews: %w", err)
	}

	// Generate BlurHash
	slog.Info("Reading original image and generating BlurHash")
	originalImage, _, err := image.Decode(bytes.NewReader(fileBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode original image: %w", err)
	}

	slog.Info("Generating BlurHash")
	hash, err := blurhash.Encode(4, 3, originalImage)
	if err != nil {
		return nil, fmt.Errorf("failed to generate BlurHash: %w", err)
	}
	slog.Info("Generated BlurHash", "hash", hash)

	// Prepare metadata for database insertion
	slog.Info("Preparing metadata for database insertion")
	photo := Photo{
		ID:        id,
		Name:      header.Filename,
		BlurHash:  hash,
		Width:     originalImage.Bounds().Dx(),
		Height:    originalImage.Bounds().Dy(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store metadata in the database
	slog.Info("Storing metadata in the database", "id", id)
	err = s.insertPhoto(ctx, &photo)
	if err != nil {
		return nil, fmt.Errorf("failed to store photo metadata in database: %w", err)
	}

	slog.Info("Photo uploaded successfully", "id", id)
	return &photo, nil
}

func (s *PhotoService) GetPhoto(ctx context.Context, id string, quality string) (io.ReadCloser, error) {
	filename := quality + ".jpg"
	item, err := s.storage.StorageRepo.GetItem(ctx, "photos", id, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s photo: %w", quality, err)
	}

	return s.storage.StorageRepo.GetItemContent(ctx, item)
}

func (s *PhotoService) DeletePhoto(ctx context.Context, id string) error {
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

func (s *PhotoService) generateAndStorePreviews(ctx context.Context, fileBytes []byte, id string) error {
	src, err := imaging.Decode(bytes.NewReader(fileBytes))
	if err != nil {
		return fmt.Errorf("failed to decode original image: %w", err)
	}

	type Preview struct {
		resizedImage *image.NRGBA
		name         string
		width        int
		format       imaging.Format
		mimeType     string
	}

	previews := []Preview{
		{name: "large.jpg", width: 1920, format: imaging.JPEG, mimeType: "image/jpeg"},
		{name: "medium.jpg", width: 768, format: imaging.JPEG, mimeType: "image/jpeg"},
		{name: "small.jpg", width: 300, format: imaging.JPEG, mimeType: "image/jpeg"},
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(previews))

	for _, preview := range previews {
		wg.Add(1)
		go func(preview Preview) {
			defer wg.Done()

			slog.Info("Generating preview", "name", preview.name)
			// Resize image
			resized := imaging.Resize(src, preview.width, 0, imaging.Lanczos)

			// Encode image to buffer
			var buf bytes.Buffer
			if err := imaging.Encode(&buf, resized, preview.format); err != nil {
				errCh <- fmt.Errorf("failed to encode %s preview: %w", preview.name, err)
				return
			}

			// Store preview in storage
			slog.Info("Storing preview", "name", preview.name)
			if _, err := s.storage.StorageRepo.PutItem(ctx, "photos", id, preview.name, &buf, int64(buf.Len()), preview.mimeType); err != nil {
				errCh <- fmt.Errorf("failed to store %s preview: %w", preview.name, err)
				return
			}
		}(preview)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *PhotoService) GetPhotos(ctx context.Context) ([]Photo, error) {
	slog.Info("Fetching photos from database")
	photos, err := s.getAllPhotos(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos from database: %w", err)
	}
	return photos, nil
}

func (s *PhotoService) insertPhoto(ctx context.Context, photo *Photo) error {
	slog.Info("Inserting photo into database", "id", photo.ID)
	query := `
		INSERT INTO photos (id, name, blur_hash, width, height, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.Pool.Exec(ctx, query, photo.ID, photo.Name, photo.BlurHash, photo.Width, photo.Height, photo.CreatedAt, photo.UpdatedAt)
	return err
}

func (s *PhotoService) getAllPhotos(ctx context.Context) ([]Photo, error) {
	query := `
		SELECT id, name, blur_hash, width, height, created_at, updated_at
		FROM photos
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