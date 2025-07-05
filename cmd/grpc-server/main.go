package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

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
	// Create cancellable context for graceful shutdown
	bgCtx := context.Background()
	globalCtx, cancel := signal.NotifyContext(bgCtx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Instance()
	cfg := config.Instance()

	isProduction := os.Getenv("ENV") == "production"

	logger.Info(globalCtx, cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
		slog.Bool("gracefulShutdown", isProduction),
	)

	// Initialize telemetry (OpenTelemetry + Pyroscope)
	shutdown, _ := tracer.Instance(globalCtx)
	defer shutdown()

	// Connect to MongoDB
	db, err := database.Instance(globalCtx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		logger.Error(globalCtx, "Failed to connect to MongoDB", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Wiring
	productRepo := repository.NewProductRepository(db.Database)
	productService := service.NewProductService(productRepo)
	productHandler := grpcHandler.NewProductGRPCHandler(productService)

	// Start gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware_grpc.UnaryTracingInterceptor()),
	)
	pb.RegisterProductServiceServer(grpcServer, productHandler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.AppPort)
	if err != nil {
		logger.Error(globalCtx, "failed to listen", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info(globalCtx, "gRPC server running", slog.String("port", cfg.AppPort))

	// Run gRPC server in background
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error(globalCtx, "failed to serve", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-globalCtx.Done()

	if !isProduction {
		logger.Info(globalCtx, "Received shutdown signal, exiting immediately")
		os.Exit(0)
	} else {
		logger.Info(globalCtx, "Shutting down gRPC server")
		grpcServer.GracefulStop()
		logger.Info(globalCtx, "gRPC server exited cleanly")
	}
}
