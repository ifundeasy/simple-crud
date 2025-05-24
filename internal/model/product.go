package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type Product struct {
	ID    primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name  string             `json:"name" bson:"name"`
	Price float64            `json:"price" bson:"price"`
	Stock int                `json:"stock" bson:"stock"`
}
