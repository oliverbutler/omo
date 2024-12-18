package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"oliverbutler/lib/environment"
	"time"

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

var (
	Tracer trace.Tracer
	tp     *sdktrace.TracerProvider
)

func newOTLPExporter(ctx context.Context) (oteltrace.SpanExporter, error) {
	// Change default HTTPS -> HTTP
	insecureOpt := otlptracehttp.WithInsecure()

	// Update default OTLP reciver endpoint
	endpointOpt := otlptracehttp.WithEndpoint("10.0.0.40:4318")

	return otlptracehttp.New(ctx, insecureOpt, endpointOpt)
}

func newTraceProvider(env *environment.EnvironmentService, exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("omo"),
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

	tp = newTraceProvider(env, exp)

	otel.SetTracerProvider(tp)

	Tracer = tp.Tracer("omo")

	return nil
}

func Teardown() {
	_ = tp.Shutdown(context.Background())
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
			ctx, span := Tracer.Start(ctx, name, trace.WithAttributes(
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
