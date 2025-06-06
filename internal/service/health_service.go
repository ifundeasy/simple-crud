package service

import (
	"context"
	"time"

	"simple-crud/internal/logger"

	"go.opentelemetry.io/otel"

	"go.mongodb.org/mongo-driver/mongo"
)

type HealthService struct {
	Mongo *mongo.Client
}

type HealthStatus struct {
	Mongo string
}

var HealthServiceTracer = otel.Tracer("HealthService")

func NewHealthService(mongo *mongo.Client) *HealthService {
	return &HealthService{
		Mongo: mongo,
	}
}

func (s *HealthService) Check(ctx context.Context) HealthStatus {
	ctx, span := HealthServiceTracer.Start(ctx, "HealthService.GetAll")
	defer span.End()
	logger.Info(ctx, "Service")

	status := HealthStatus{Mongo: "UP"}

	// MongoDB
	mongoCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.Mongo.Ping(mongoCtx, nil); err != nil {
		status.Mongo = "DOWN"
	}

	return status
}
