package storage

import (
	"context"
	"io"
	"oliverbutler/lib/environment"
	"os"
	"path/filepath"
)

const LocalStorageRoot = "/home/olly/Desktop/omo"

// FileItem represents a file in the storage system
type FileItem struct {
	Name        string
	Size        int64
	ContentType string
	Folder      string
	Bucket      string
}

// StorageRepo defines the interface for storage operations
type StorageRepo interface {
	PutItem(ctx context.Context, bucket, folder string, name string, reader io.Reader, size int64, contentType string) (*FileItem, error)
	GetItem(ctx context.Context, bucket, folder, name string) (*FileItem, error)
	DeleteItem(ctx context.Context, bucket, folder, name string) error
	DeleteFolder(ctx context.Context, bucket, folder string) error
	ListItems(ctx context.Context, bucket, folder string) ([]*FileItem, error)
	GetItemContent(ctx context.Context, item *FileItem) (io.ReadCloser, error)
}

// LocalStorageRepo implements StorageRepo for local file system
type LocalStorageRepo struct{}

type StorageService struct {
	StorageRepo StorageRepo
}

func NewStorageService(env *environment.EnvironmentService) (*StorageService, error) {
	storageRepo := NewLocalStorageRepo()
	return &StorageService{StorageRepo: storageRepo}, nil
}

// NewLocalStorageRepo creates a new LocalStorageRepo
func NewLocalStorageRepo() *LocalStorageRepo {
	return &LocalStorageRepo{}
}

// PutItem uploads an item to the specified bucket and folder
func (l *LocalStorageRepo) PutItem(ctx context.Context, bucket, folder string, name string, reader io.Reader, size int64, contentType string) (*FileItem, error) {
	fullPath := filepath.Join(LocalStorageRoot, bucket, folder, name)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return nil, err
	}

	return &FileItem{
		Name:        name,
		Size:        size,
		ContentType: contentType,
		Folder:      folder,
		Bucket:      bucket,
	}, nil
}

// GetItem retrieves metadata for an item
func (l *LocalStorageRepo) GetItem(ctx context.Context, bucket, folder, name string) (*FileItem, error) {
	fullPath := filepath.Join(LocalStorageRoot, bucket, folder, name)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	return &FileItem{
		Name:        name,
		Size:        info.Size(),
		ContentType: "", // Local storage doesn't store content type
		Folder:      folder,
		Bucket:      bucket,
	}, nil
}

// DeleteItem removes an item from storage
func (l *LocalStorageRepo) DeleteItem(ctx context.Context, bucket, folder, name string) error {
	fullPath := filepath.Join(LocalStorageRoot, bucket, folder, name)
	return os.Remove(fullPath)
}

// ListItems lists all items in a folder
func (l *LocalStorageRepo) ListItems(ctx context.Context, bucket, folder string) ([]*FileItem, error) {
	var items []*FileItem
	fullPath := filepath.Join(LocalStorageRoot, bucket, folder)

	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(fullPath, path)
			items = append(items, &FileItem{
				Name:        info.Name(),
				Size:        info.Size(),
				ContentType: "", // Local storage doesn't store content type
				Folder:      filepath.Dir(relPath),
				Bucket:      bucket,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return items, nil
}

// GetItemContent retrieves the content of an item
func (l *LocalStorageRepo) GetItemContent(ctx context.Context, item *FileItem) (io.ReadCloser, error) {
	fullPath := filepath.Join(LocalStorageRoot, item.Bucket, item.Folder, item.Name)
	return os.Open(fullPath)
}

func (l *LocalStorageRepo) DeleteFolder(ctx context.Context, bucket, folder string) error {
	fullPath := filepath.Join(LocalStorageRoot, bucket, folder)
	return os.RemoveAll(fullPath)
}
