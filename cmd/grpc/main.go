package main

import (
	"context"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"simple-crud/internal/config"
	"simple-crud/internal/database"
	grpcHandler "simple-crud/internal/handler/grpc"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/logger"
	middleware_grpc "simple-crud/internal/middleware/grpc"
	"simple-crud/internal/repository"
	"simple-crud/internal/service"
	"simple-crud/internal/tracer"
	"simple-crud/internal/version"
)

func main() {
	globalCtx := context.Background()
	log := logger.Instance()
	cfg := config.Instance()

	log.Info(cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
	)

	// Initialize telemetry (OpenTelemetry + Pyroscope)
	shutdown, _ := tracer.Instance(globalCtx)
	defer shutdown()

	// Connect to MongoDB
	db, err := database.Instance(globalCtx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		log.Error("Failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}

	// Wiring
	productRepo := repository.NewProductRepository(db.Database)
	productService := service.NewProductService(productRepo)
	productHandler := grpcHandler.NewProductGRPCHandler(productService)

	// Start gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware_grpc.UnaryTracingInterceptor(globalCtx)),
	)
	pb.RegisterProductServiceServer(grpcServer, productHandler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.AppPort)
	if err != nil {
		log.Error("failed to listen: %v", err)
	}

	log.Info("gRPC server running", "port", cfg.AppPort)

	if err := grpcServer.Serve(lis); err != nil {
		log.Error("failed to serve: %v", err)
	}
}
