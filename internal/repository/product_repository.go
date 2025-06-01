package repository

import (
	"context"
	"log/slog"

	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"simple-crud/internal/model"
	"simple-crud/internal/utils"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProductRepository struct {
	collection *mongo.Collection
}

func NewProductRepository(db *mongo.Database) *ProductRepository {
	return &ProductRepository{
		collection: db.Collection("product"),
	}
}

func (s *ProductRepository) logging(span trace.Span, function string, method string, filter string, payload *model.Product) {
	log := logger.Instance()
	log.Info("Repository",
		slog.String("trace_id", span.SpanContext().TraceID().String()),
		slog.String("span_id", span.SpanContext().SpanID().String()),
		slog.String("function", function),
		slog.String("method", method),
		slog.String("filter", filter),
		slog.String("payload", utils.ToJSONString(payload)),
	)
}

func (r *ProductRepository) Insert(ctx context.Context, product *model.Product) error {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "InsertProductRepository")
	defer span.End()

	r.logging(span, "InsertProductRepository", "InsertOne", "", product)
	product.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(ctx, product)
	return err
}

func (r *ProductRepository) FindAll(ctx context.Context) ([]model.Product, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "getProductsRepository")
	defer span.End()

	r.logging(span, "getProductsRepository", "Find", "", nil)

	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var products []model.Product
	for cursor.Next(ctx) {
		var product model.Product
		if err := cursor.Decode(&product); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, nil
}

func (r *ProductRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Product, error) {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "getProductByIdRepository")
	defer span.End()

	r.logging(span, "getProductByIdRepository", "FindOne", id.Hex(), nil)

	var product model.Product
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&product)
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) Update(ctx context.Context, id primitive.ObjectID, updated *model.Product) error {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "updateProductByIdRepository")
	defer span.End()

	r.logging(span, "updateProductByIdRepository", "UpdateOne", id.Hex(), nil)

	update := bson.M{
		"$set": bson.M{
			"name":  updated.Name,
			"price": updated.Price,
			"stock": updated.Stock,
		},
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *ProductRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	cfg := config.Instance()
	tracer := otel.Tracer(cfg.AppName)
	_, span := tracer.Start(ctx, "deleteProductByIdRepository")
	defer span.End()

	r.logging(span, "deleteProductByIdRepository", "DeleteOne", id.Hex(), nil)

	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
