package http

import (
	"encoding/json"
	"net/http"

	"simple-crud/internal/logger"
	"simple-crud/internal/model"
	"simple-crud/internal/service"

	"go.opentelemetry.io/otel"
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
	ctx, span := HttpProductHandlerTracer.Start(r.Context(), "HttpProductHandler.GetAll")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler")

	products, err := h.service.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(products)
}

func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx, span := HttpProductHandlerTracer.Start(r.Context(), "HttpProductHandler.GetByID")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	product, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(product)
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx, span := HttpProductHandlerTracer.Start(r.Context(), "HttpProductHandler.Create")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler")

	var product model.Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	created, err := h.service.Create(r.Context(), &product)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx, span := HttpProductHandlerTracer.Start(r.Context(), "HttpProductHandler.Update")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler")

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

	if err := h.service.Update(r.Context(), id, &p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully"})
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx, span := HttpProductHandlerTracer.Start(r.Context(), "HttpProductHandler.Delete")
	defer span.End()
	logger.Info(ctx, "HttpProductHandler")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Product deleted successfully"})
}
