package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"oliverbutler/gpx"
	"oliverbutler/pages"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
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
