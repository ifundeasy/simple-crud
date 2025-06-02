package middleware_http

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"simple-crud/internal/logger"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("HttpMiddleware")

func TraceMiddleware(globalCtx context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
			defer span.End()

			var body []byte
			if r.Body != nil {
				body, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(io.NopCloser(io.MultiReader(bytes.NewBuffer(body))))
			}

			logger.Info(ctx, "HttpMiddleware",
				slog.String("http.method", r.Method),
				slog.String("http.path", r.URL.Path),
				slog.String("http.query", r.URL.RawQuery),
				slog.String("http.remote", r.RemoteAddr),
				slog.String("http.body", string(body)),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
