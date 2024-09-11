package storage

import (
	"context"
	"io"
	"oliverbutler/lib/environment"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

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

// MinioStorageRepo implements StorageRepo for MinIO
type MinioStorageRepo struct {
	client *minio.Client
}

type StorageService struct {
	StorageRepo StorageRepo
}

func NewStorageService(env *environment.EnvironmentService) (*StorageService, error) {
	storageRepo, err := NewMinioStorageRepo(env.GetMinioEndpoint(), env.GetMinioAccessKey(), env.GetMinioSecretKey(), env.GetMinioUseSSL())
	if err != nil {
		return nil, err
	}

	return &StorageService{StorageRepo: storageRepo}, nil
}

// NewMinioStorageRepo creates a new MinioStorageRepo
func NewMinioStorageRepo(endpoint, accessKeyID, secretAccessKey string, useSSL bool) (*MinioStorageRepo, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinioStorageRepo{client: client}, nil
}

// PutItem uploads an item to the specified bucket and folder
func (m *MinioStorageRepo) PutItem(ctx context.Context, bucket, folder string, name string, reader io.Reader, size int64, contentType string) (*FileItem, error) {
	objectName := filepath.Join(folder, name)

	// Ensure bucket exists
	err := m.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	if err != nil {
		// Check if the bucket already exists
		exists, errBucketExists := m.client.BucketExists(ctx, bucket)
		if errBucketExists == nil && exists {
			// Bucket already exists, continue
		} else {
			return nil, err
		}
	}

	_, err = m.client.PutObject(ctx, bucket, objectName, reader, size, minio.PutObjectOptions{ContentType: contentType})
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
func (m *MinioStorageRepo) GetItem(ctx context.Context, bucket, folder, name string) (*FileItem, error) {
	objectName := filepath.Join(folder, name)
	info, err := m.client.StatObject(ctx, bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}

	return &FileItem{
		Name:        name,
		Size:        info.Size,
		ContentType: info.ContentType,
		Folder:      folder,
		Bucket:      bucket,
	}, nil
}

// DeleteItem removes an item from storage
func (m *MinioStorageRepo) DeleteItem(ctx context.Context, bucket, folder, name string) error {
	objectName := filepath.Join(folder, name)
	return m.client.RemoveObject(ctx, bucket, objectName, minio.RemoveObjectOptions{})
}

// ListItems lists all items in a folder
func (m *MinioStorageRepo) ListItems(ctx context.Context, bucket, folder string) ([]*FileItem, error) {
	var items []*FileItem

	objectCh := m.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    folder,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}

		items = append(items, &FileItem{
			Name:        filepath.Base(object.Key),
			Size:        object.Size,
			ContentType: object.ContentType,
			Folder:      filepath.Dir(object.Key),
			Bucket:      bucket,
		})
	}

	return items, nil
}

// GetItemContent retrieves the content of an item
func (m *MinioStorageRepo) GetItemContent(ctx context.Context, item *FileItem) (io.ReadCloser, error) {
	objectName := filepath.Join(item.Folder, item.Name)
	return m.client.GetObject(ctx, item.Bucket, objectName, minio.GetObjectOptions{})
}

func (m *MinioStorageRepo) DeleteFolder(ctx context.Context, bucket, folder string) error {
	objectCh := m.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    folder,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return object.Err
		}

		if err := m.client.RemoveObject(ctx, bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	return nil
}
