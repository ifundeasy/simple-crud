package http

import (
	"context"
	"io"
	"net/http"

	"log/slog"

	"simple-crud/internal/config"
	"simple-crud/internal/logger"

	"go.opentelemetry.io/otel"
)

type ExternalHandler struct {
	URL string
}

var ExternalHandlerTracer = otel.Tracer("ExternalHandler")

func NewExternalHandler(url string) *ExternalHandler {
	return &ExternalHandler{
		URL: url,
	}
}

func (h *ExternalHandler) Fetch(globalCtx context.Context, w http.ResponseWriter, r *http.Request) {
	ctx, span := ExternalHandlerTracer.Start(r.Context(), "ExternalHandler.Fetch")
	defer span.End()
	logger.Info(ctx, "Handler")

	cfg := config.Instance()
	resp, err := http.Get(cfg.ExternalHTTP + "/products")
	if err != nil {
		logger.Error(ctx, "Failed to call external service", slog.String("error", err.Error()))
		http.Error(w, "External call failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error(ctx, "Failed to read response", slog.String("error", err.Error()))
		http.Error(w, "Error reading external response", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error(ctx, "External service returned non-200", slog.Int("status_code", resp.StatusCode))
		http.Error(w, "External error", resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}
