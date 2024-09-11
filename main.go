package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"oliverbutler/lib"
	"oliverbutler/lib/environment"
	"oliverbutler/pages"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	_ "github.com/lib/pq"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	app, err := lib.NewApp()
	if err != nil {
		slog.Error("Failed to create app", "error", err)
		return
	}

	fileServer := http.FileServer(http.Dir("./static"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.Environment.GetEnv() == environment.Local {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		fileServer.ServeHTTP(w, r)
	})

	r.Handle("/static/*", http.StripPrefix("/static/", handler))

	InitDevReloadWebsocket(r)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		pages.Index(context.TODO(), app).Render(w)
	})

	r.Get("/post/{slug}", func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		pages.Post(context.TODO(), app, slug).Render(w)
	})

	r.Get("/photos", func(w http.ResponseWriter, r *http.Request) {
		pages.Photos(context.TODO(), app).Render(w)
	})

	r.Get("/hikes", func(w http.ResponseWriter, r *http.Request) {
		pages.MapsPage(context.TODO(), app).Render(w)
	})

	host := "0.0.0.0"

	if app.Environment.GetEnv() == environment.Local {
		host = "localhost"
	}

	addr := host + ":6900"

	slog.Info(fmt.Sprintf("Starting server on %s", addr))
	http.ListenAndServe(addr, r)
}
