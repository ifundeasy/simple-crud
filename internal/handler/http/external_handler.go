package http

import (
	"net/http"
	"time"

	"log/slog"

	"simple-crud/internal/client"
	"simple-crud/internal/config"
	"simple-crud/internal/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
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

func (h *ExternalHandler) Fetch(w http.ResponseWriter, r *http.Request) {
	parentCtx := r.Context()

	// Start span with extracted context
	// Extract context from incoming headers (traceparent, etc.)
	propCtx := otel.GetTextMapPropagator().Extract(parentCtx, propagation.HeaderCarrier(r.Header))
	ctx, span := ExternalHandlerTracer.Start(propCtx, "HttpExternalHandler.Fetch")
	defer span.End()
	logger.Info(ctx, "HttpExternalHandler.Fetch")

	// Call external HTTP
	cfg := config.Instance()
	httpClient := client.NewHTTPClient(cfg.ExternalHTTP, 3*time.Second)

	resp, err := httpClient.GetWithResponse("/products", client.RequestOptions{
		Context: ctx,
	})
	if err != nil {
		return
	}

	logger.Info(ctx, "Fetched products", slog.Int("count", len(resp.RawBody)))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp.RawBody)
}
