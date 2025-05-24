package database

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

type Mongo struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func Connect(ctx context.Context, logger *slog.Logger, uri, dbName string) (*Mongo, error) {
	opts := options.Client().
		ApplyURI(uri).
		SetMonitor(otelmongo.NewMonitor())

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		logger.Error("Failed to connect to MongoDB", slog.String("error", err.Error()))
		return nil, err
	}

	// Ping with timeout context
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		logger.Error("MongoDB ping failed", slog.String("error", err.Error()))
		return nil, err
	}

	logger.Info("Connected to MongoDB successfully")

	return &Mongo{
		Client:   client,
		Database: client.Database(dbName),
	}, nil
}
