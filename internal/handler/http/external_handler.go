package http

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"log/slog"

	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"simple-crud/internal/utils"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type ExternalHandler struct {
	URL string
}

func NewExternalHandler(url string) *ExternalHandler {
	return &ExternalHandler{
		URL: url,
	}
}

var log = logger.Instance()

func (h *ExternalHandler) logging(span trace.Span, function string, r *http.Request) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(io.NopCloser(io.MultiReader(bytes.NewBuffer(body))))
	}
	log.Info("HTTP Request",
		slog.String("trace_id", span.SpanContext().TraceID().String()),
		slog.String("span_id", span.SpanContext().SpanID().String()),
		slog.String("function", function),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("query", r.URL.RawQuery),
		slog.String("remote", r.RemoteAddr),
		slog.String("hostname", utils.GetHost()),
		slog.String("body", string(body)),
	)
}

func (h *ExternalHandler) Fetch(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("external-api")
	_, span := tracer.Start(r.Context(), "fetchExternalHandler")
	defer span.End()

	h.logging(span, "fetchExternalHandler", r)

	cfg := config.Instance()
	resp, err := http.Get(cfg.ExternalHTTP + "/products")
	if err != nil {
		log.Error("Failed to call external service", slog.String("error", err.Error()))
		http.Error(w, "External call failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Failed to read response", slog.String("error", err.Error()))
		http.Error(w, "Error reading external response", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Error("External service returned non-200", slog.Int("status_code", resp.StatusCode))
		http.Error(w, "External error", resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}
