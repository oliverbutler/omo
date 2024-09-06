package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"oliverbutler/components"
	"os"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pressly/goose/v3"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq" // Import the PostgreSQL driver for `database/sql`
)

var (
	tripCache      []Trip
	tripCacheMutex sync.RWMutex
)

func main() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")

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

	defer pool.Close()

	// Load the cache on startup
	if err := loadTripCache(); err != nil {
		slog.Error("Failed to load trip cache", "error", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	fileServer := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	InitDevReloadWebsocket(r)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		components.Page(components.DebugBody()).Render(w)
	})

	r.Get("/maps", func(w http.ResponseWriter, r *http.Request) {
		tripCacheMutex.RLock()
		// tripsJSON, err := json.Marshal(tripCache)
		tripCacheMutex.RUnlock()

		w.Write([]byte(fmt.Sprintf(`maps page with %d trips`, len(tripCache))))
	})

	addr := "0.0.0.0:6900"

	slog.Info(fmt.Sprintf("Starting server on %s", addr))
	http.ListenAndServe(addr, r)
}
