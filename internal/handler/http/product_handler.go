package http

import (
	"encoding/json"
	"net/http"

	"simple-crud/internal/logger"
	"simple-crud/internal/model"
	"simple-crud/internal/service"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type ProductHandler struct {
	service *service.ProductService
}

var HttpProductHandlerTracer = otel.Tracer("HttpProductHandler")

func NewProductHandler(service *service.ProductService) *ProductHandler {
	return &ProductHandler{
		service: service,
	}
}

func (h *ProductHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	parentCtx := r.Context()

	// Start span with extracted context
	// Extract context from incoming headers (traceparent, etc.)
	propCtx := otel.GetTextMapPropagator().Extract(parentCtx, propagation.HeaderCarrier(r.Header))
	ctx, span := HttpProductHandlerTracer.Start(propCtx, "HttpProductHandler.GetAll")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler.GetAll")

	products, err := h.service.GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(products)
}

func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	parentCtx := r.Context()

	// Start span with extracted context
	// Extract context from incoming headers (traceparent, etc.)
	propCtx := otel.GetTextMapPropagator().Extract(parentCtx, propagation.HeaderCarrier(r.Header))
	ctx, span := HttpProductHandlerTracer.Start(propCtx, "HttpProductHandler.GetByID")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler.GetByID")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	product, err := h.service.GetByID(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(product)
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	parentCtx := r.Context()

	// Start span with extracted context
	// Extract context from incoming headers (traceparent, etc.)
	propCtx := otel.GetTextMapPropagator().Extract(parentCtx, propagation.HeaderCarrier(r.Header))
	ctx, span := HttpProductHandlerTracer.Start(propCtx, "HttpProductHandler.Create")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler.Create")

	var product model.Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	created, err := h.service.Create(ctx, &product)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(created)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	parentCtx := r.Context()

	// Start span with extracted context
	// Extract context from incoming headers (traceparent, etc.)
	propCtx := otel.GetTextMapPropagator().Extract(parentCtx, propagation.HeaderCarrier(r.Header))
	ctx, span := HttpProductHandlerTracer.Start(propCtx, "HttpProductHandler.Update")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler.Update")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	var p model.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := h.service.Update(ctx, id, &p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully"})
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	parentCtx := r.Context()

	// Start span with extracted context
	// Extract context from incoming headers (traceparent, etc.)
	propCtx := otel.GetTextMapPropagator().Extract(parentCtx, propagation.HeaderCarrier(r.Header))
	ctx, span := HttpProductHandlerTracer.Start(propCtx, "HttpProductHandler.Delete")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler.Delete")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Product deleted successfully"})
}
