package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"oliverbutler/gpx"
	"oliverbutler/pages"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pressly/goose/v3"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq" // Import the PostgreSQL driver for `database/sql`
)

var (
	tripCache      atomic.Value
	tripCacheReady = make(chan struct{})
)

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Error("Failed to load .env file", "error", err)
	}

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	env := os.Getenv("ENV")

	// MinIO configuration
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	minioUseSSL := os.Getenv("MINIO_USE_SSL") == "true"

	// Initialize MinIO client
	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: minioUseSSL,
	})
	if err != nil {
		slog.Error("Failed to create MinIO client", "error", err)
		return
	}

	// Test MinIO connection by creating a file and appending timestamp
	err = testMinioConnection(minioClient)
	if err != nil {
		slog.Error("Failed to test MinIO connection", "error", err)
		return
	}

	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		return
	}
	defer pool.Close()

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		slog.Error("Failed to convert pgxpool to *sql.DB", "error", err)
		return
	}
	defer db.Close()

	gooseProvider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("./migrations"))

	res, err := gooseProvider.Up(context.Background())
	if err != nil {
		slog.Error("Failed to run migrations", "error", err)
		panic(err)
	}

	if res != nil {
		slog.Info("Migrations ran successfully")

		for _, r := range res {
			slog.Info(fmt.Sprintf("Migration: %s in %s", r.String(), r.Duration.String()))
		}
	}

	// Start loading the cache asynchronously
	go func() {
		if err := loadTripCache(); err != nil {
			slog.Error("Failed to load trip cache", "error", err)
		}
		close(tripCacheReady)
	}()

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// Create a custom file server handler
	fileServer := http.FileServer(http.Dir("./static"))

	// Wrap the file server with a custom handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if env == "local" {
			// Set no-cache headers for all static assets in local environment
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		fileServer.ServeHTTP(w, r)
	})

	// Use the custom handler with Chi
	r.Handle("/static/*", http.StripPrefix("/static/", handler))

	InitDevReloadWebsocket(r)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		pages.Index(context.TODO()).Render(w)
	})

	r.Get("/post/{slug}", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		pages.Post(context.TODO(), slug).Render(w)
	})

	r.Get("/photos", func(w http.ResponseWriter, r *http.Request) {
		pages.Photos(context.TODO()).Render(w)
	})

	r.Get("/hikes", handleHikesPage)

	host := "0.0.0.0"

	if env == "local" {
		host = "localhost"
	}

	addr := host + ":6900"

	slog.Info(fmt.Sprintf("Starting server on %s", addr))
	http.ListenAndServe(addr, r)
}

func handleHikesPage(w http.ResponseWriter, r *http.Request) {
	select {
	case <-tripCacheReady:
		// Cache is ready, proceed
	case <-time.After(5 * time.Second):
		// Timeout if cache takes too long to load
		http.Error(w, "Cache is still loading, please try again later", http.StatusServiceUnavailable)
		return
	}

	trips := tripCache.Load().([]gpx.Trip)

	pages.MapsPage(trips).Render(w)
}

func loadTripCache() error {
	// Simulating cache loading
	time.Sleep(2 * time.Second)

	trips, err := gpx.ReadTripData()
	if err != nil {
		return err
	}

	tripCache.Store(trips)
	return nil
}

func testMinioConnection(minioClient *minio.Client) error {
	bucketName := "test-bucket"
	objectName := "test.txt"
	location := "us-east-1"

	// Create bucket if it doesn't exist
	err := minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		// Check to see if we already own this bucket
		exists, errBucketExists := minioClient.BucketExists(context.Background(), bucketName)
		if errBucketExists == nil && exists {
			slog.Info("Bucket already exists", "bucket", bucketName)
		} else {
			return err
		}
	} else {
		slog.Info("Bucket created successfully", "bucket", bucketName)
	}

	// Prepare content
	timestamp := time.Now().Format(time.RFC3339)
	content := fmt.Sprintf("Test file created at: %s\n", timestamp)

	// Check if the file exists and read its content
	obj, err := minioClient.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err == nil {
		existingContent, err := io.ReadAll(obj)
		if err == nil {
			content = string(existingContent) + content
		}
	}

	slog.Info("Latest content", "content", content)

	// Upload the file
	_, err = minioClient.PutObject(context.Background(), bucketName, objectName, strings.NewReader(content), int64(len(content)), minio.PutObjectOptions{ContentType: "text/plain"})
	if err != nil {
		return err
	}

	slog.Info("File uploaded successfully", "bucket", bucketName, "object", objectName)
	return nil
}
