package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"simple-crud/internal/config"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/logger"
	"simple-crud/internal/telemetry"
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

	isProduction := os.Getenv("ENV") == "production"

	logger.Info(
		globalCtx,
		"Starting gRPC client",
		slog.String("service.name", cfg.AppName),
		slog.String("service.version", version.Version),
		slog.String("service.git_version", version.Commit),
		slog.String("service.build_time", version.BuildTime),
		slog.Bool("service.gracefull_shutdown", isProduction),
	)

	shutdown, _ := telemetry.Instance(globalCtx)
	defer shutdown()

	GrpcRequestorTracer := otel.Tracer("GrpcRequestorMain")

	conn, err := grpc.NewClient(
		cfg.ExternalGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		logger.Error(globalCtx, "Failed to connect to gRPC server",
			slog.String("grpc.remote_addr", cfg.ExternalGRPC),
			slog.String("exception.message", err.Error()),
			slog.String("exception.type", fmt.Sprintf("%T", errors.Unwrap(err))),
			slog.String("exception.stacktrace", string(debug.Stack())),
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
		slog.String("data.target", cfg.ExternalGRPC),
		slog.Int("data.max_client_delay", int(cfg.ClientMaxSleepMs)),
		slog.Int("data.dns_resolver_delay", int(cfg.DnsResolverDelayMs)),
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
			ctx, span := GrpcRequestorTracer.Start(ctx, "backend-grpc-request")

			// Extract trace ID from span context
			spanContext := span.SpanContext()
			traceID := spanContext.TraceID().String()

			// Create metadata with trace ID
			md := metadata.Pairs("x-trace-id", traceID)
			ctx = metadata.NewOutgoingContext(ctx, md)

			var trailer metadata.MD

			resp, err := client.GetAll(ctx, &emptypb.Empty{}, grpc.Trailer(&trailer))
			cancel()
			span.End()

			// Extract trace-id from trailer (for server response)
			serverTraceIDs := trailer.Get("x-trace-id")
			var serverTraceID string
			if len(serverTraceIDs) < 1 {
				serverTraceID = "empty"
				logger.Warn(ctx, "No Trace ID received from server")
			} else {
				serverTraceID = serverTraceIDs[0]
			}

			if err != nil {
				logger.Error(ctx, "Error calling GetAll",
					slog.String("grpc.trailers.trace_id", serverTraceID),
					slog.String("exception.message", err.Error()),
					slog.String("exception.type", fmt.Sprintf("%T", errors.Unwrap(err))),
					slog.String("exception.stacktrace", string(debug.Stack())),
				)
			} else {
				logger.Info(
					ctx,
					"Fetched products",
					slog.Int("data.count", len(resp.GetProducts())),
					slog.String("grpc.resolver", resp.Resolver),
					slog.String("grpc.trailers.trace_id", serverTraceID),
				)
			}

			delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
			time.Sleep(delay)
		}
	}
}
