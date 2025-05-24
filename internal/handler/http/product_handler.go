package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"log/slog"
	"simple-crud/internal/model"
	"simple-crud/internal/service"
	"simple-crud/pkg/logger"
)

type ProductHandler struct {
	service *service.ProductService
	logger  *slog.Logger
}

func NewProductHandler(service *service.ProductService, logger *slog.Logger) *ProductHandler {
	return &ProductHandler{
		service: service,
		logger:  logger,
	}
}

func (h *ProductHandler) logRequest(r *http.Request) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(io.NopCloser(io.MultiReader(bytes.NewBuffer(body))))
	}
	h.logger.Info("HTTP Request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("query", r.URL.RawQuery),
		slog.String("remote", r.RemoteAddr),
		slog.String("hostname", logger.Hostname),
		slog.String("body", string(body)),
	)
}

func (h *ProductHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	h.logRequest(r)

	products, err := h.service.GetAll(r.Context())
	if err != nil {
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(products)
}

func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	h.logRequest(r)

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
	h.logRequest(r)

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
	h.logRequest(r)

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

	if err := h.service.Update(r.Context(), id, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully"})
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	h.logRequest(r)

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
