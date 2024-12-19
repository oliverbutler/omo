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
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/go-chi/chi/v5/middleware"

	_ "github.com/lib/pq"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

func newOTLPExporter(ctx context.Context) (oteltrace.SpanExporter, error) {
	// Change default HTTPS -> HTTP
	insecureOpt := otlptracehttp.WithInsecure()

	// Update default OTLP reciver endpoint
	endpointOpt := otlptracehttp.WithEndpoint("10.0.0.40:4318")

	return otlptracehttp.New(ctx, insecureOpt, endpointOpt)
}

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("omo"),
		),
	)
	if err != nil {
		panic(err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

func NewOpenTelemetryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()

			// Extract tracing information from the incoming request
			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

			name := fmt.Sprintf("%s %s", r.Method, r.URL.String())

			// Start a new span
			ctx, span := tracing.Tracer.Start(ctx, name, trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
			))
			defer span.End()

			// Create a custom ResponseWriter to capture the status code and size
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Pass the span context to the next handler
			r = r.WithContext(ctx)

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Log after the request finishes
			duration := time.Since(start)
			logger.InfoContext(ctx, "Request completed",
				slog.String("method", r.Method),
				slog.String("url", r.URL.String()),
				slog.Int("status", rw.statusCode),
				slog.Int("responseSize", rw.size),
				slog.Duration("duration", duration),
			)
		})
	}
}

func main() {
	ctx := context.Background()
	r := chi.NewRouter()

	exp, err := newOTLPExporter(ctx)
	if err != nil {
		slog.Error("Failed to create console exporter", "error", err)
		return
	}

	tp := newTraceProvider(exp)

	defer func() { _ = tp.Shutdown(ctx) }()

	otel.SetTracerProvider(tp)

	tracing.Tracer = tp.Tracer("omo")

	logging.OmoLogger = logging.NewOmoLogger(slog.NewJSONHandler(os.Stdout, nil))

	app, err := lib.NewApp()
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

	r.Use(middleware.Recoverer, NewOpenTelemetryMiddleware(logging.OmoLogger))

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
