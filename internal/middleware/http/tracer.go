package middleware_http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"log/slog"
	"simple-crud/internal/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("HttpMiddleware")

// TraceMiddleware wraps HTTP handlers with OpenTelemetry tracing.
// It captures request & response metadata, injects trace ID into response headers,
// handles panics safely, and logs enriched request data.
func TraceMiddleware(globalCtx context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
			defer func() {
				if rec := recover(); rec != nil {
					span.RecordError(errFromRecover(rec))
					span.SetStatus(codes.Error, "panic occurred")
					panic(rec)
				}
				span.End()
			}()

			r = r.WithContext(ctx)

			// Read request body (safe for re-read downstream)
			var body []byte
			if r.Body != nil {
				body, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(body))
			}

			// Wrap response writer to capture status code and size
			rw := &responseWriter{ResponseWriter: w, statusCode: 200}
			start := time.Now()

			// Extract TraceID early
			traceID := span.SpanContext().TraceID().String()
			// Inject trace ID into response header BEFORE handler runs
			rw.Header().Set("X-Trace-ID", traceID)

			// Call actual handler
			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			// Enrich span attributes after handler completes
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.target", r.URL.Path),
				attribute.String("http.query", r.URL.RawQuery),
				attribute.String("http.remote_addr", r.RemoteAddr),
				attribute.Int("http.status_code", rw.statusCode),
				attribute.Int64("http.response_content_length", rw.size),
				attribute.Int64("http.duration_ms", duration.Milliseconds()),
			)

			// Set OpenTelemetry span status
			if rw.statusCode >= 500 {
				span.SetStatus(codes.Error, "internal server error")
			} else if rw.statusCode >= 400 {
				span.SetStatus(codes.Error, "client error")
			} else {
				span.SetStatus(codes.Ok, "")
			}

			// Inject trace ID into application logs
			logger.Info(ctx, "HttpMiddleware",
				slog.String("trace_id", traceID),
				slog.String("http.method", r.Method),
				slog.String("http.path", r.URL.Path),
				slog.String("http.query", r.URL.RawQuery),
				slog.String("http.remote", r.RemoteAddr),
				slog.String("http.body", string(body)),
				slog.Int("http.status", rw.statusCode),
				slog.Int64("duration_ms", duration.Milliseconds()),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and response size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)
	return n, err
}

// errFromRecover converts panic value into error for span recording.
func errFromRecover(rec interface{}) error {
	if err, ok := rec.(error); ok {
		return err
	}
	return &panicError{rec}
}

// panicError implements error interface for non-error panic values.
type panicError struct {
	value interface{}
}

func (p *panicError) Error() string {
	return "panic: " + stringify(p.value)
}

func stringify(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	default:
		return "unknown panic"
	}
}
