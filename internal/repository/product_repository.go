package repository

import (
	"context"

	"simple-crud/internal/model"

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

func (r *ProductRepository) Insert(ctx context.Context, product *model.Product) error {
	product.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(ctx, product)
	return err
}

func (r *ProductRepository) FindAll(ctx context.Context) ([]model.Product, error) {
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
	var product model.Product
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&product)
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) Update(ctx context.Context, id primitive.ObjectID, updated model.Product) error {
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
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
