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
	r.Use(middleware.Logger, middleware.Recoverer)

	app, err := lib.NewApp()
	if err != nil {
		slog.Error("Failed to create app", "error", err)
		return
	}

	defer app.Database.Pool.Close()

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
		user, _ := app.Users.ExtractUserFromCookies(w, r)

		pages.Index(context.TODO(), app, user).Render(w)
	})

	r.Get("/post/{slug}", func(w http.ResponseWriter, r *http.Request) {
		user, _ := app.Users.ExtractUserFromCookies(w, r)

		slug := chi.URLParam(r, "slug")
		pages.Post(context.TODO(), app, user, slug).Render(w)
	})

	r.Get("/photos", func(w http.ResponseWriter, r *http.Request) {
		user, _ := app.Users.ExtractUserFromCookies(w, r)

		pages.Photos(context.TODO(), app, user).Render(w)
	})

	r.Get("/hikes", func(w http.ResponseWriter, r *http.Request) {
		pages.MapsPage(context.TODO(), app).Render(w)
	})

	r.Get("/api/auth/github/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")

		userSessionResponse, err := app.Users.HandleGithubAuthCallback(code)
		if err != nil {
			slog.Error("HandleGithubAuthCallback failed", "error", err)
			pages.Error(context.TODO(), err).Render(w)
			return
		}
		// Set cookies
		http.SetCookie(w, &http.Cookie{
			Name:     "AccessToken",
			Value:    userSessionResponse.AccessToken,
			MaxAge:   1800,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "RefreshToken",
			Value:    userSessionResponse.RefreshToken,
			MaxAge:   10000,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "UserSessionId",
			Value:    userSessionResponse.UserSessionId,
			MaxAge:   10000,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})

		// Redirect
		http.Redirect(w, r, "/", http.StatusFound)
	})

	host := "0.0.0.0"

	if app.Environment.GetEnv() == environment.Local {
		host = "localhost"
	}

	addr := host + ":6900"

	slog.Info(fmt.Sprintf("Starting server on %s", addr))
	http.ListenAndServe(addr, r)
}
