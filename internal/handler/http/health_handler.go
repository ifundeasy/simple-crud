package http

import (
	"context"
	"encoding/json"
	"net/http"

	"simple-crud/internal/logger"
	"simple-crud/internal/service"

	"go.opentelemetry.io/otel"
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

func (h *HealthHandler) Check(globalCtx context.Context, w http.ResponseWriter, r *http.Request) {
	ctx, span := HttpHealthHandlerTracer.Start(r.Context(), "HttpHealthHandler.GetAll")
	defer span.End()
	logger.Info(ctx, "HttpHealthHandler")

	status := h.service.Check(r.Context())

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
	_ = json.NewEncoder(w).Encode(resp)
}
