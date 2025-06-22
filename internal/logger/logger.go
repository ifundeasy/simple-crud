package logger

import (
	"context"
	"log/slog"
	"os"
	"simple-crud/internal/utils"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

var (
	instance *slog.Logger
	once     sync.Once
)

func Instance() *slog.Logger {
	once.Do(func() {
		instance = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
			// AddSource: true,
		}))
	})

	return instance
}

func Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	enrichedAttrs := enrich(ctx, attrs...)
	Instance().Info(msg, attrsToArgs(enrichedAttrs)...)
	sendLog("info", msg, enrichedAttrs)
}

func Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	enrichedAttrs := enrich(ctx, attrs...)
	Instance().Warn(msg, attrsToArgs(enrichedAttrs)...)
	sendLog("warn", msg, enrichedAttrs)
}

func Error(ctx context.Context, msg string, attrs ...slog.Attr) {
	enrichedAttrs := enrich(ctx, attrs...)
	Instance().Error(msg, attrsToArgs(enrichedAttrs)...)
	sendLog("error", msg, enrichedAttrs)
}

func enrich(ctx context.Context, attrs ...slog.Attr) []slog.Attr {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		attrs = append(attrs,
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
			slog.String("hostname", utils.GetHost()),
		)
	}

	return attrs
}

// Convert slog.Attr to slog's variadic ...any
func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	return args
}
