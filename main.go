package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"oliverbutler/components"
	"oliverbutler/lib"
	"oliverbutler/lib/environment"
	"oliverbutler/lib/logging"
	"oliverbutler/lib/tracing"
	"oliverbutler/pages"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/go-chi/chi/v5/middleware"

	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()
	r := chi.NewRouter()

	app, err := lib.NewApp(ctx)
	if err != nil {
		slog.Error("Failed to create app", "error", err)
		return
	}

	defer app.TearDown()

	fileServer := http.FileServer(http.Dir("./static"))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if app.Environment.GetEnv() == environment.Local {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		fileServer.ServeHTTP(w, r)
	})

	r.Use(middleware.Recoverer, tracing.NewOpenTelemetryMiddleware(logging.OmoLogger))

	r.Handle("/static/*", http.StripPrefix("/static/", handler))

	InitDevReloadWebsocket(r)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		logging.OmoLogger.InfoContext(r.Context(), "User visiting home page")

		ctx := r.Context()
		user, _ := app.Users.ExtractUserFromCookies(ctx, w, r)

		pages.Index(ctx, app, user).Render(w)
	})

	r.Get("/post/{slug}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, _ := app.Users.ExtractUserFromCookies(ctx, w, r)

		slug := chi.URLParam(r, "slug")
		pages.Post(ctx, app, user, slug).Render(w)
	})

	r.Get("/photos", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, _ := app.Users.ExtractUserFromCookies(ctx, w, r)

		pages.Photos(ctx, app, user).Render(w)
	})

	r.Get("/photos/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, _ := app.Users.ExtractUserFromCookies(ctx, w, r)

		id := chi.URLParam(r, "id")

		pages.PhotoPage(ctx, app, user, id).Render(w)
	})

	r.Get("/photos/manage", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, _ := app.Users.ExtractUserFromCookies(ctx, w, r)

		if user.IsLoggedIn == false || user.User.Email != "dev@oliverbutler.uk" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		pages.PhotosManage(ctx, app, user).Render(w)
	})

	r.Post("/photos/upload", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, _ := app.Users.ExtractUserFromCookies(ctx, w, r)

		if user.IsLoggedIn == false || user.User.Email != "dev@oliverbutler.uk" {
			slog.Warn("Unauthorized user tried to upload photos")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		err := app.Photos.UploadPhotosAndStartWorkflows(r.Context(), r)
		if err != nil {
			slog.Error("Failed to upload photos", "error", err)
			http.Error(w, "Failed to upload photos", http.StatusInternalServerError)
			return
		}

		components.SucceessBanner("Photos uploaded successfully").Render(w)
	})

	r.Get("/api/photos/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
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
		photo, err := app.Photos.GetPhotoBuffer(ctx, id, quality)
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
		ctx := r.Context()
		user, _ := app.Users.ExtractUserFromCookies(ctx, w, r)

		if user.IsLoggedIn == false || user.User.Email != "dev@oliverbutler.uk" {
			slog.Warn("Unauthorized user tried to delete photo")
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		id := chi.URLParam(r, "id")

		err := app.Photos.DeletePhoto(ctx, id)
		if err != nil {
			slog.Error("Failed to delete photo", "error", err)
			http.Redirect(w, r, "/photos/manage", http.StatusFound)
			return
		}

		slog.Info("Photo deleted")
	})

	r.Get("/hikes", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		pages.MapsPage(ctx, app).Render(w)
	})

	r.Get("/api/auth/github/callback", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		code := r.URL.Query().Get("code")

		userSessionResponse, err := app.Users.HandleGithubAuthCallback(ctx, code)
		if err != nil {
			slog.Error("HandleGithubAuthCallback failed", "error", err)
			pages.Error(ctx, err).Render(w)
			return
		}

		secureCookie := strings.HasPrefix(app.Environment.GetBaseURL(), "https")

		// Set cookies
		http.SetCookie(w, &http.Cookie{
			Name:     "AccessToken",
			Value:    userSessionResponse.AccessToken,
			MaxAge:   1800,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   secureCookie,
			HttpOnly: true,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "RefreshToken",
			Value:    userSessionResponse.RefreshToken,
			MaxAge:   10000,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   secureCookie,
			HttpOnly: true,
		})

		http.SetCookie(w, &http.Cookie{
			Name:     "UserSessionId",
			Value:    userSessionResponse.UserSessionId,
			MaxAge:   10000,
			Path:     "/",
			Domain:   app.Environment.GetDomain(),
			Secure:   secureCookie,
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

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	host := "0.0.0.0"

	if app.Environment.GetEnv() == environment.Local {
		host = "localhost"
	}

	addr := host + ":6900"

	logging.OmoLogger.Info(fmt.Sprintf("Starting server on %s", addr))
	http.ListenAndServe(addr, r)
}
