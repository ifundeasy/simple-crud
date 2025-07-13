package service

import (
	"context"
	"errors"

	"simple-crud/internal/logger"
	"simple-crud/internal/model"
	"simple-crud/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
)

type ProductService struct {
	repo *repository.ProductRepository
}

var ProductServiceTracer = otel.Tracer("ProductService")

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) Create(ctx context.Context, p *model.Product) (*model.Product, error) {
	ctx, span := ProductServiceTracer.Start(ctx, "ProductService.Create")
	defer span.End()
	logger.Info(ctx, "ProductService.Create")

	if p.Name == "" || p.Price <= 0 || p.Stock < 0 {
		return nil, errors.New("invalid product data")
	}
	err := s.repo.Insert(ctx, p)
	return p, err
}

func (s *ProductService) GetAll(ctx context.Context) ([]model.Product, error) {
	ctx, span := ProductServiceTracer.Start(ctx, "ProductService.GetAll")
	defer span.End()
	logger.Info(ctx, "ProductService.GetAll")

	return s.repo.FindAll(ctx)
}

func (s *ProductService) GetByID(ctx context.Context, id string) (*model.Product, error) {
	ctx, span := ProductServiceTracer.Start(ctx, "ProductService.GetByID")
	defer span.End()
	logger.Info(ctx, "ProductService.GetByID")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid ID format")
	}
	return s.repo.FindByID(ctx, objID)
}

func (s *ProductService) Update(ctx context.Context, id string, p *model.Product) error {
	ctx, span := ProductServiceTracer.Start(ctx, "ProductService.Update")
	defer span.End()
	logger.Info(ctx, "ProductService.Update")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid ID format")
	}
	return s.repo.Update(ctx, objID, p)
}

func (s *ProductService) Delete(ctx context.Context, id string) error {
	ctx, span := ProductServiceTracer.Start(ctx, "ProductService.Delete")
	defer span.End()
	logger.Info(ctx, "ProductService.Delete")

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid ID format")
	}
	return s.repo.Delete(ctx, objID)
}
