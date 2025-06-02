package database

import (
	"context"
	"log/slog"
	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

type Mongo struct {
	Client   *mongo.Client
	Database *mongo.Database
}

var (
	instance *Mongo
	once     sync.Once
)

func Instance(globalCtx context.Context, uri, dbName string) (*Mongo, error) {
	var err error

	once.Do(func() {
		var cfg = config.Instance()

		_uri := uri
		if _uri == "" {
			_uri = cfg.MongoURI
		}

		opts := options.Client().
			ApplyURI(_uri).
			SetMonitor(otelmongo.NewMonitor())

		var log = logger.Instance()
		client, connErr := mongo.Connect(globalCtx, opts)
		if connErr != nil {
			log.Error("Failed to connect to MongoDB", slog.String("error", connErr.Error()))
			err = connErr
			return
		}

		// Ping with timeout context
		pingCtx, cancel := context.WithTimeout(globalCtx, 5*time.Second)
		defer cancel()
		if pingErr := client.Ping(pingCtx, nil); pingErr != nil {
			log.Error("MongoDB ping failed", slog.String("error", pingErr.Error()))
			err = pingErr
			return
		}

		log.Info("Connected to MongoDB successfully")

		_dbName := dbName
		if _dbName == "" {
			_dbName = cfg.MongoDBName
		}
		instance = &Mongo{
			Client:   client,
			Database: client.Database(_dbName),
		}
	})

	return instance, err
}
