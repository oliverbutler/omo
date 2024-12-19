package logging

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type traceContextHandler struct {
	slog.Handler
}

func (h *traceContextHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.Add(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func NewTraceContextHandler(handler slog.Handler) slog.Handler {
	return &traceContextHandler{Handler: handler}
}

func NewOmoLogger(handler slog.Handler) *slog.Logger {
	return slog.New(NewTraceContextHandler(handler))
}

var OmoLogger *slog.Logger
