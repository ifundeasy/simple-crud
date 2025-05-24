package main

import (
	"context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"simple-crud/internal/config"
	"simple-crud/internal/database"
	grpcHandler "simple-crud/internal/handler/grpc"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/repository"
	"simple-crud/internal/service"
	"simple-crud/internal/telemetry"
	"simple-crud/pkg/logger"
)

func main() {
	ctx := context.Background()
	logg := logger.New()

	// Load config
	cfg := config.Load(logg)

	// Initialize telemetry (OpenTelemetry + Pyroscope)
	shutdown := telemetry.Init(ctx, logg, cfg)
	defer shutdown()

	// Connect to MongoDB
	db, err := database.Connect(ctx, logg, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		logg.Error("Failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}

	// Wiring
	productRepo := repository.NewProductRepository(db.Database)
	productService := service.NewProductService(productRepo)
	productHandler := grpcHandler.NewProductGRPCHandler(productService, logg)

	// Start gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterProductServiceServer(grpcServer, productHandler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.AppPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	logg.Info("gRPC server running", "port", cfg.AppPort)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
