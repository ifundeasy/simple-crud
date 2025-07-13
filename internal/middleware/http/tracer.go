package middleware_http

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"simple-crud/internal/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

var tracer = otel.Tracer("HttpMiddleware")

type HeaderWrapper struct {
	http.Header
}

// ResponseWriter captures status, size **and body**.
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
	buf        bytes.Buffer // 🆕 holds the response body (up to MaxBodyLogged)
}

func (rw *ResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *ResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)

	if rw.buf.Len() < logger.MaxBodyLogged {
		// Copy into buffer, but never exceed MaxBodyLogged
		toCopy := logger.MaxBodyLogged - rw.buf.Len()
		if len(b) < toCopy {
			toCopy = len(b)
		}
		rw.buf.Write(b[:toCopy])
	}
	return n, err
}

// TraceMiddleware wraps HTTP handlers with OpenTelemetry tracing.
// It captures request & response metadata, injects trace ID into response headers,
// handles panics safely, and logs enriched request data.
func TraceMiddleware(globalCtx context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request headers
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start span with the extracted context (will continue existing trace if present)
			ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path)
			defer func() {
				if rec := recover(); rec != nil {
					span.RecordError(errFromRecover(rec))
					span.SetStatus(codes.Error, "panic occurred")
					panic(rec)
				}
				span.End()
			}()

			attrs := logger.LogHTTPRequest(ctx, r, "incoming::request")
			logger.Info(ctx, "HTTP", attrs...)

			// Wrap response writer to capture status code and size
			rw := &ResponseWriter{ResponseWriter: w, statusCode: 200}
			start := time.Now()

			// Extract TraceID early
			traceID := span.SpanContext().TraceID().String()

			// Inject trace ID into response header BEFORE handler runs
			rw.Header().Set("X-Trace-ID", traceID)
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))

			// Call actual handler
			next.ServeHTTP(rw, r)

			// Set OpenTelemetry span status
			if rw.statusCode >= 500 {
				span.SetStatus(codes.Error, "internal server error")
			} else if rw.statusCode >= 400 {
				span.SetStatus(codes.Error, "client error")
			} else {
				span.SetStatus(codes.Ok, "")
			}

			duration := time.Since(start)

			attrs = logger.LogHTTPResponse(ctx, r, rw.Header(), rw.statusCode, &rw.buf, duration.Milliseconds(), "incoming::response")
			logger.Info(ctx, "HTTP", attrs...)
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
