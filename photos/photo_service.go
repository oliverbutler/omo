package photos

import (
	"context"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bbrks/go-blurhash"
	"github.com/disintegration/imaging"
	"gopkg.in/yaml.v2"
)

type PhotoService struct {
	logger *slog.Logger
	mu     sync.Mutex
}

func NewPhotoService() *PhotoService {
	return &PhotoService{}
}

type Photo struct {
	Name      string
	Path      string
	LargePath string
	ThumbPath string
	BlurHash  string
	Width     int
	Height    int
}

type PhotoMeta struct {
	BlurHash string `yaml:"blurhash"`
	Width    int    `yaml:"width"`
	Height   int    `yaml:"height"`
}

func (s *PhotoService) GetPhotos(ctx context.Context) ([]Photo, error) {
	photoDir := "./static/photos/originals"
	previewDir := "./static/photos/previews"
	photos := []Photo{}

	slog.Info("Starting to get photos", "photoDir", photoDir)

	files, err := os.ReadDir(photoDir)
	if err != nil {
		slog.Error("Failed to read photo directory", "error", err)
		return nil, err
	}

	var wg sync.WaitGroup
	for _, file := range files {
		if !file.IsDir() {
			ext := strings.ToLower(filepath.Ext(file.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" {
				originalPath := filepath.Join(photoDir, file.Name())
				largePath := filepath.Join(previewDir, file.Name()+".large"+ext)
				thumbPath := filepath.Join(previewDir, file.Name()+".thumb"+ext)
				metaPath := filepath.Join(previewDir, file.Name()+".meta.yaml")

				photo := Photo{
					Name:      file.Name(),
					Path:      originalPath,
					LargePath: largePath,
					ThumbPath: thumbPath,
				}

				// Check if previews exist, if not, generate them in the background
				if _, err := os.Stat(largePath); os.IsNotExist(err) {
					wg.Add(1)
					go func(p Photo) {
						defer wg.Done()
						s.generatePreviewsWithLock(p.Path, p.LargePath, p.ThumbPath, metaPath)
					}(photo)
				}

				// Load metadata
				meta, err := s.loadMetadata(metaPath)
				if err != nil {
					slog.Error("Failed to load metadata", "file", file.Name(), "error", err)
					continue
				}

				photo.BlurHash = meta.BlurHash
				photo.Width = meta.Width
				photo.Height = meta.Height

				photos = append(photos, photo)
			}
		}
	}

	// Wait for all background tasks to complete
	wg.Wait()

	slog.Info("Finished getting photos", "count", len(photos))
	return photos, nil
}

func (s *PhotoService) generatePreviewsWithLock(originalPath, largePath, thumbPath, metaPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check again if previews exist, as they might have been generated while waiting for the lock
	if _, err := os.Stat(largePath); !os.IsNotExist(err) {
		return
	}

	slog.Info("Generating previews", "originalPath", originalPath)

	if err := s.generatePreviews(originalPath, largePath, thumbPath, metaPath); err != nil {
		slog.Error("Failed to generate previews", "originalPath", originalPath, "error", err)
	}
}

func (s *PhotoService) generatePreviews(originalPath, largePath, thumbPath, metaPath string) error {
	// Open the original image
	src, err := imaging.Open(originalPath)
	if err != nil {
		return err
	}

	// Generate large preview (1MB target size)
	large := imaging.Resize(src, 1920, 0, imaging.Lanczos)
	if err := imaging.Save(large, largePath); err != nil {
		return err
	}

	// Generate thumbnail (300px width)
	thumb := imaging.Resize(src, 300, 0, imaging.Lanczos)
	if err := imaging.Save(thumb, thumbPath); err != nil {
		return err
	}

	// Generate BlurHash
	hash, err := blurhash.Encode(4, 3, src)
	if err != nil {
		return err
	}

	// Save metadata
	meta := PhotoMeta{
		BlurHash: hash,
		Width:    src.Bounds().Dx(),
		Height:   src.Bounds().Dy(),
	}

	metaFile, err := os.Create(metaPath)
	if err != nil {
		return err
	}
	defer metaFile.Close()

	encoder := yaml.NewEncoder(metaFile)
	return encoder.Encode(meta)
}

func (s *PhotoService) loadMetadata(metaPath string) (*PhotoMeta, error) {
	slog.Info("Loading metadata", "metaPath", metaPath)

	metaFile, err := os.Open(metaPath)
	if err != nil {
		slog.Error("Failed to open metadata file", "metaPath", metaPath, "error", err)
		return nil, err
	}
	defer metaFile.Close()

	var meta PhotoMeta
	decoder := yaml.NewDecoder(metaFile)
	if err := decoder.Decode(&meta); err != nil {
		slog.Error("Failed to decode metadata", "metaPath", metaPath, "error", err)
		return nil, err
	}

	return &meta, nil
}
