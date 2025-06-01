package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"log/slog"
	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"simple-crud/internal/model"
	"simple-crud/internal/service"
	"simple-crud/internal/utils"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type ProductHandler struct {
	service *service.ProductService
}

func NewProductHandler(service *service.ProductService) *ProductHandler {
	return &ProductHandler{
		service: service,
	}
}

func (h *ProductHandler) logging(span trace.Span, function string, r *http.Request) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(io.NopCloser(io.MultiReader(bytes.NewBuffer(body))))
	}
	log := logger.Instance()
	log.Info("HTTP",
		slog.String("trace_id", span.SpanContext().TraceID().String()),
		slog.String("span_id", span.SpanContext().SpanID().String()),
		slog.String("hostname", utils.GetHost()),
		slog.String("function", function),
		slog.String("http.method", r.Method),
		slog.String("http.path", r.URL.Path),
		slog.String("http.query", r.URL.RawQuery),
		slog.String("http.remote", r.RemoteAddr),
		slog.String("http.body", string(body)),
	)
}

func (h *ProductHandler) GetAll(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "getProductsHandler")
	defer span.End()

	h.logging(span, "getProductsHandler", r)

	products, err := h.service.GetAll(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(products)
}

func (h *ProductHandler) GetByID(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "getProductByIdHandler")
	defer span.End()

	h.logging(span, "getProductByIdHandler", r)

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
	_ = json.NewEncoder(w).Encode(product)
}

func (h *ProductHandler) Create(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "createProductHandler")
	defer span.End()

	h.logging(span, "createProductHandler", r)

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
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (h *ProductHandler) Update(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "updateProductByIdHandler")
	defer span.End()

	h.logging(span, "updateProductByIdHandler", r)

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
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully"})
}

func (h *ProductHandler) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "deleteProductByIdHandler")
	defer span.End()

	h.logging(span, "deleteProductByIdHandler", r)

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Product deleted successfully"})
}
