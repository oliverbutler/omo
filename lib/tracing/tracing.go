package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"oliverbutler/lib/environment"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	oteltrace "go.opentelemetry.io/otel/sdk/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	OmoTracer        trace.Tracer
	OmoTraceProvider *sdktrace.TracerProvider
	DBTracer         trace.Tracer
	DBTraceProvider  *sdktrace.TracerProvider
)

func GetSpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func newOTLPExporter(ctx context.Context) (oteltrace.SpanExporter, error) {
	// Change default HTTPS -> HTTP
	insecureOpt := otlptracehttp.WithInsecure()

	// Update default OTLP reciver endpoint
	endpointOpt := otlptracehttp.WithEndpoint("10.0.0.40:4318")

	return otlptracehttp.New(ctx, insecureOpt, endpointOpt)
}

func newTraceProvider(name string, env *environment.EnvironmentService, exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(name),
			semconv.DeploymentEnvironment(env.GetEnv().String()),
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

func InitTracing(ctx context.Context, env *environment.EnvironmentService) error {
	exp, err := newOTLPExporter(ctx)
	if err != nil {
		slog.Error("Failed to create console exporter", "error", err)
		return err
	}

	OmoTraceProvider = newTraceProvider("omo", env, exp)
	otel.SetTracerProvider(OmoTraceProvider)
	OmoTracer = OmoTraceProvider.Tracer("omo")

	DBTraceProvider = newTraceProvider("omo-db", env, exp)
	otel.SetTracerProvider(DBTraceProvider)
	DBTracer = DBTraceProvider.Tracer("omodb")

	return nil
}

func Teardown() {
	_ = OmoTraceProvider.Shutdown(context.Background())
	_ = DBTraceProvider.Shutdown(context.Background())
}

// https://github.com/go-chi/chi/issues/270#issuecomment-479184559
func getRoutePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if pattern := rctx.RoutePattern(); pattern != "" {
		// Pattern is already available
		return pattern
	}

	routePath := r.URL.Path
	if r.URL.RawPath != "" {
		routePath = r.URL.RawPath
	}

	tctx := chi.NewRouteContext()
	if !rctx.Routes.Match(tctx, r.Method, routePath) {
		// No matching pattern, so just return the request path.
		// Depending on your use case, it might make sense to
		// return an empty string or error here instead
		return routePath
	}

	// tctx has the updated pattern, since Match mutates it
	return tctx.RoutePattern()
}

func NewOpenTelemetryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()

			// if hitting /ws skip this and just call the next handler
			if r.URL.Path == "/ws" {
				next.ServeHTTP(w, r)
				return
			}

			// Extract tracing information from the incoming request
			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

			name := fmt.Sprintf("%s %s", r.Method, getRoutePattern(r))

			// Start a new span
			ctx, span := OmoTracer.Start(ctx, name, trace.WithAttributes(
				attribute.String(string(semconv.HTTPRequestMethodKey), r.Method),
				semconv.HTTPRoute(r.URL.Path),
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
			logger.InfoContext(ctx, fmt.Sprintf("Responded to %s", name),
				slog.String("method", r.Method),
				slog.String("url", r.URL.String()),
				slog.Int("status", rw.statusCode),
				slog.Int("responseSize", rw.size),
				slog.Duration("duration", duration),
			)

			if rw.statusCode >= 400 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", rw.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}

			span.SetAttributes(
				semconv.HTTPResponseStatusCode(rw.statusCode),
				semconv.HTTPResponseSize(rw.size),
				semconv.HTTPRoute(r.URL.Path),
			)
		})
	}
}
