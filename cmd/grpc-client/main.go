package main

import (
	"context"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"simple-crud/internal/config"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/logger"
	"simple-crud/internal/version"

	"go.opentelemetry.io/otel"
)

func main() {
	// Create cancellable context for graceful shutdown
	bgCtx := context.Background()
	globalCtx, stop := signal.NotifyContext(bgCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Instance()
	cfg := config.Instance()
	tracer := otel.Tracer("backend-grpc-client")

	isProduction := os.Getenv("ENV") == "production"

	logger.Info(
		globalCtx,
		cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
		slog.Bool("gracefulShutdown", isProduction),
	)

	conn, err := grpc.NewClient(
		cfg.ExternalGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		logger.Error(
			globalCtx,
			"Failed to connect to gRPC server",
			slog.String("error", err.Error()),
			slog.String("target", cfg.ExternalGRPC),
		)
		os.Exit(1)
	}
	defer func() {
		logger.Info(globalCtx, "Closing gRPC connection")
		_ = conn.Close()
	}()

	client := pb.NewProductServiceClient(conn)

	logger.Info(
		globalCtx,
		"gRPC client started",
		slog.String("target", cfg.ExternalGRPC),
		slog.Int("max_client_delay", int(cfg.ClientMaxSleepMs)),
		slog.Int("dns_resolver_delay", int(cfg.DnsResolverDelayMs)),
	)

	for {
		select {
		case <-globalCtx.Done():
			if !isProduction {
				logger.Info(globalCtx, "Received shutdown signal, exiting immediately")
				os.Exit(0)
			} else {
				logger.Info(globalCtx, "Shutting down gRPC client")
				return
			}

		default:
			// Add span tracing
			ctx, cancel := context.WithTimeout(globalCtx, 3*time.Second)
			ctx, span := tracer.Start(ctx, "backend-grpc-request")
			var trailer metadata.MD

			resp, err := client.GetAll(ctx, &emptypb.Empty{}, grpc.Trailer(&trailer))
			cancel()
			span.End()

			// Extract trace-id from trailer
			traceIDs := trailer.Get("x-trace-id")
			var traceID string
			if len(traceIDs) < 1 {
				traceID = "empty"
				logger.Warn(ctx, "No Trace ID received")
			} else {
				traceID = traceIDs[0]
			}

			if err != nil {
				logger.Error(
					ctx,
					"Error calling GetAll",
					slog.String("error", err.Error()),
					slog.String("trace_id", traceID),
				)
			} else {
				logger.Info(
					ctx,
					"Received products",
					slog.String("resolver", resp.Resolver),
					slog.String("trace_id", traceID),
					slog.Int("count", len(resp.GetProducts())),
				)
			}

			delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
			time.Sleep(delay)
		}
	}
}
