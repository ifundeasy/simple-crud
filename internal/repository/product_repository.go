package repository

import (
	"context"

	"simple-crud/internal/logger"
	"simple-crud/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/otel"
)

type ProductRepository struct {
	collection *mongo.Collection
}

var ProductRepositoryTracer = otel.Tracer("ProductRepository")

func NewProductRepository(db *mongo.Database) *ProductRepository {
	return &ProductRepository{
		collection: db.Collection("product"),
	}
}

func (r *ProductRepository) Insert(ctx context.Context, product *model.Product) error {
	ctx, span := ProductRepositoryTracer.Start(ctx, "ProductRepository.Insert")
	defer span.End()
	logger.Info(ctx, "Repository")

	product.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(ctx, product)
	return err
}

func (r *ProductRepository) FindAll(ctx context.Context) ([]model.Product, error) {
	ctx, span := ProductRepositoryTracer.Start(ctx, "ProductRepository.FindAll")
	defer span.End()
	logger.Info(ctx, "Repository")

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
	ctx, span := ProductRepositoryTracer.Start(ctx, "ProductRepository.FindByID")
	defer span.End()
	logger.Info(ctx, "Repository")

	var product model.Product
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&product)
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) Update(ctx context.Context, id primitive.ObjectID, updated *model.Product) error {
	ctx, span := ProductRepositoryTracer.Start(ctx, "ProductRepository.Update")
	defer span.End()
	logger.Info(ctx, "Repository")

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
	ctx, span := ProductRepositoryTracer.Start(ctx, "ProductRepository.Delete")
	defer span.End()
	logger.Info(ctx, "Repository")

	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
