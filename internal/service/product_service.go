package service

import (
	"context"
	"errors"
	"log/slog"

	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"simple-crud/internal/model"
	"simple-crud/internal/repository"
	"simple-crud/internal/utils"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProductService struct {
	repo *repository.ProductRepository
}

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) logging(span trace.Span, function string, filter string, payload *model.Product) {
	log := logger.Instance()
	log.Info("Service Level",
		slog.String("trace_id", span.SpanContext().TraceID().String()),
		slog.String("span_id", span.SpanContext().SpanID().String()),
		slog.String("function", function),
		slog.String("filter", filter),
		slog.String("payload", utils.ToJSONString(payload)),
	)
}

func (s *ProductService) Create(ctx context.Context, p *model.Product) (*model.Product, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "createProductService")
	defer span.End()

	s.logging(span, "getProductsService", "", p)

	if p.Name == "" || p.Price <= 0 || p.Stock < 0 {
		return nil, errors.New("invalid product data")
	}
	err := s.repo.Insert(ctx, p)
	return p, err
}

func (s *ProductService) GetAll(ctx context.Context) ([]model.Product, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "getProductsService")
	defer span.End()

	s.logging(span, "getProductsService", "", nil)

	return s.repo.FindAll(ctx)
}

func (s *ProductService) GetByID(ctx context.Context, id string) (*model.Product, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "getProductByIdService")
	defer span.End()

	s.logging(span, "getProductByIdService", id, nil)

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid ID format")
	}
	return s.repo.FindByID(ctx, objID)
}

func (s *ProductService) Update(ctx context.Context, id string, p *model.Product) error {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "updateProductByIdService")
	defer span.End()

	s.logging(span, "updateProductByIdService", id, p)

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid ID format")
	}
	return s.repo.Update(ctx, objID, p)
}

func (s *ProductService) Delete(ctx context.Context, id string) error {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "deleteProductByIdService")
	defer span.End()

	s.logging(span, "deleteProductByIdService", id, nil)

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid ID format")
	}
	return s.repo.Delete(ctx, objID)
}
