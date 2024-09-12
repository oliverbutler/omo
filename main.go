package main

import (
	"context"
	"fmt"
	"io"
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

	r.Get("/photos/manage", func(w http.ResponseWriter, r *http.Request) {
		user, _ := app.Users.ExtractUserFromCookies(w, r)

		if user.IsLoggedIn == false || user.User.Email != "dev@oliverbutler.uk" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		pages.PhotosManage(context.TODO(), app, user).Render(w)
	})

	r.Post("/photos/upload", func(w http.ResponseWriter, r *http.Request) {
		user, _ := app.Users.ExtractUserFromCookies(w, r)

		if user.IsLoggedIn == false || user.User.Email != "dev@oliverbutler.uk" {
			slog.Warn("Unauthorized user tried to upload photo")
			return
		}

		photo, err := app.Photos.UploadPhoto(context.TODO(), r)
		if err != nil {
			slog.Error("Failed to upload photo", "error", err)
			return
		}

		slog.Info("Photo uploaded")

		pages.PhotoManageTile(photo).Render(w)
	})

	r.Get("/api/photos/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		// Parse the quality query parameter
		quality := r.URL.Query().Get("quality")
		if quality == "" {
			quality = "original"
		}

		// Validate quality value
		validQualities := map[string]bool{
			"original": true,
			"large":    true,
			"medium":   true,
			"small":    true,
		}
		if !validQualities[quality] {
			quality = "original"
		}

		// Retrieve the photo based on quality
		photo, err := app.Photos.GetPhoto(r.Context(), id, quality)
		if err != nil {
			slog.Error("Failed to get photo", "error", err)
			pages.Error(r.Context(), err).Render(w)
			return
		}
		defer photo.Close() // Ensure the ReadCloser is closed

		// Set appropriate headers before sending the photo.
		// You may need to set Content-Type and Content-Disposition headers here.
		w.Header().Set("Content-Type", "image/jpeg")                            // Adjust based on photo type
		w.Header().Set("Content-Disposition", "inline; filename=\"photo.jpg\"") // Optional; set filename appropriately

		// Copy the photo data to the response writer
		if _, err := io.Copy(w, photo); err != nil {
			slog.Error("Failed to write photo to response", "error", err)
			pages.Error(r.Context(), err).Render(w)
			return
		}
	})

	r.Delete("/photos/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, _ := app.Users.ExtractUserFromCookies(w, r)

		if user.IsLoggedIn == false || user.User.Email != "dev@oliverbutler.uk" {
			slog.Warn("Unauthorized user tried to delete photo")
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		id := chi.URLParam(r, "id")

		err := app.Photos.DeletePhoto(context.TODO(), id)
		if err != nil {
			slog.Error("Failed to delete photo", "error", err)
			http.Redirect(w, r, "/photos/manage", http.StatusFound)
			return
		}

		slog.Info("Photo deleted")
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

	r.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
		slog.Info("User logging out")

		http.SetCookie(w, &http.Cookie{
			Name:     "AccessToken",
			Value:    "",
			MaxAge:   0,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "RefreshToken",
			Value:    "",
			MaxAge:   0,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "UserSessionId",
			Value:    "",
			MaxAge:   0,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})

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
