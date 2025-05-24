package http

import (
	"io"
	"net/http"

	"log/slog"

	"simple-crud/pkg/logger"

	"go.opentelemetry.io/otel"
)

type ExternalHandler struct {
	URL    string
	Logger *slog.Logger
}

func NewExternalHandler(url string, logger *slog.Logger) *ExternalHandler {
	return &ExternalHandler{
		URL:    url,
		Logger: logger,
	}
}

func (h *ExternalHandler) Fetch(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("HTTP Request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("query", r.URL.RawQuery),
		slog.String("remote", r.RemoteAddr),
		slog.String("hostname", logger.Hostname),
	)

	tracer := otel.Tracer("external-api")
	_, span := tracer.Start(r.Context(), "fetchExternalData")
	defer span.End()

	resp, err := http.Get(h.URL + "/products")
	if err != nil {
		h.Logger.Error("Failed to call external service", slog.String("error", err.Error()))
		http.Error(w, "External call failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.Logger.Error("Failed to read response", slog.String("error", err.Error()))
		http.Error(w, "Error reading external response", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		h.Logger.Error("External service returned non-200", slog.Int("status_code", resp.StatusCode))
		http.Error(w, "External error", resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}
