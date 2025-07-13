package http

import (
	"encoding/json"
	"net/http"

	"simple-crud/internal/logger"
	"simple-crud/internal/service"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type HealthHandler struct {
	service *service.HealthService
}

var HttpHealthHandlerTracer = otel.Tracer("HttpHealthHandler")

func NewHealthHandler(service *service.HealthService) *HealthHandler {
	return &HealthHandler{
		service: service,
	}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	parentCtx := r.Context()

	// Start span with extracted context
	// Extract context from incoming headers (traceparent, etc.)
	propCtx := otel.GetTextMapPropagator().Extract(parentCtx, propagation.HeaderCarrier(r.Header))
	ctx, span := HttpHealthHandlerTracer.Start(propCtx, "HttpHealthHandler.Check")
	defer span.End()
	logger.Info(ctx, "HttpHealthHandler.Check")

	status := h.service.Check(propCtx)

	overall := "UP"
	if status.Mongo == "DOWN" {
		overall = "DOWN"
		w.WriteHeader(http.StatusInternalServerError)
	}

	resp := map[string]interface{}{
		"status": overall,
		"data": map[string]string{
			"mongodb": status.Mongo,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
