package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	tripCache      []Trip
	tripCacheMutex sync.RWMutex
)

func main() {
	dbUrl := os.Getenv("DB_URL")

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
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

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("You are visitor number")))
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
