package storage

import (
	"context"
	"fmt"
	"io"
	"oliverbutler/lib/environment"
	"oliverbutler/lib/tracing"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// FileItem represents a file in the storage system
type FileItem struct {
	Name        string
	Size        int64
	ContentType string
	Folder      string
	Bucket      string
	URL         string // Added URL field for direct access
}

// StorageRepo defines the interface for storage operations
type StorageRepo interface {
	PutItem(ctx context.Context, bucket, folder string, name string, reader io.Reader, size int64, contentType string) (*FileItem, error)
	GetItem(ctx context.Context, bucket, folder, name string) (*FileItem, error)
	DeleteItem(ctx context.Context, bucket, folder, name string) error
	ListItems(ctx context.Context, bucket, folder string) ([]*FileItem, error)
	GetItemContent(ctx context.Context, item *FileItem) (io.ReadCloser, error)
}

// S3StorageRepo implements StorageRepo for S3-compatible storage
type S3StorageRepo struct {
	client   *s3.Client
	endpoint string
}

type StorageService struct {
	StorageRepo StorageRepo
}

func NewStorageService(ctx context.Context, env *environment.EnvironmentService) (*StorageService, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(env.StorageAccessKeyID, env.StorageSecretAccessKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(env.StorageEndpoint)
	})

	storageRepo := &S3StorageRepo{
		client:   client,
		endpoint: "https://s3.mypacktracker.com",
	}

	return &StorageService{StorageRepo: storageRepo}, nil
}

func (s *S3StorageRepo) GetURL(folder, name string) string {
	return fmt.Sprintf("%s/%s/%s", s.endpoint, folder, name)
}

// PutItem uploads an item to the specified bucket and folder
func (s *S3StorageRepo) PutItem(ctx context.Context, bucket, folder string, name string, reader io.Reader, size int64, contentType string) (*FileItem, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "S3StorageRepo.PutItem")
	defer span.End()

	key := folder + "/" + name
	if folder == "" {
		key = name
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          reader,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &FileItem{
		Name:        name,
		Size:        size,
		ContentType: contentType,
		Folder:      folder,
		Bucket:      bucket,
		URL:         s.GetURL(folder, name),
	}, nil
}

// GetItem retrieves metadata for an item
func (s *S3StorageRepo) GetItem(ctx context.Context, bucket, folder, name string) (*FileItem, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "S3StorageRepo.GetItem",
		trace.WithAttributes(
			attribute.String("bucket", bucket),
			attribute.String("folder", folder),
			attribute.String("name", name),
		))
	defer span.End()

	key := folder + "/" + name
	if folder == "" {
		key = name
	}

	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := s.client.HeadObject(ctx, headInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	return &FileItem{
		Name:        name,
		Size:        *result.ContentLength,
		ContentType: aws.ToString(result.ContentType),
		Folder:      folder,
		Bucket:      bucket,
		URL:         s.GetURL(folder, name),
	}, nil
}

// DeleteItem removes an item from storage
func (s *S3StorageRepo) DeleteItem(ctx context.Context, bucket, folder, name string) error {
	ctx, span := tracing.OmoTracer.Start(ctx, "S3StorageRepo.DeleteItem")
	defer span.End()

	key := folder + "/" + name
	if folder == "" {
		key = name
	}

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// ListItems lists all items in a folder
func (s *S3StorageRepo) ListItems(ctx context.Context, bucket, folder string) ([]*FileItem, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "S3StorageRepo.ListItems")
	defer span.End()

	prefix := folder
	if folder != "" && !strings.HasSuffix(folder, "/") {
		prefix += "/"
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	var items []*FileItem
	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			name := filepath.Base(key)
			folderPath := filepath.Dir(key)
			if folderPath == "." {
				folderPath = ""
			}

			items = append(items, &FileItem{
				Name:        name,
				Size:        *obj.Size,
				ContentType: "", // ContentType not available in list operation
				Folder:      folderPath,
				Bucket:      bucket,
				URL:         s.GetURL(folder, name),
			})
		}
	}

	return items, nil
}

// GetItemContent retrieves the content of an item as a ReadCloser
func (s *S3StorageRepo) GetItemContent(ctx context.Context, item *FileItem) (io.ReadCloser, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "S3StorageRepo.GetItemContent",
		trace.WithAttributes(
			attribute.String("bucket", item.Bucket),
			attribute.String("folder", item.Folder),
			attribute.String("name", item.Name),
		))
	defer span.End()

	key := item.Folder + "/" + item.Name
	if item.Folder == "" {
		key = item.Name
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(item.Bucket),
		Key:    aws.String(key),
	}

	result, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object content: %w", err)
	}

	return result.Body, nil
}
