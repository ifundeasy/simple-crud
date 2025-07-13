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
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	grpcStatus "google.golang.org/grpc/status"
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

	tracer := otel.Tracer("backend-grpc-client")

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
			defer cancel()

			// --- start parent span
			ctx, span := tracer.Start(ctx, "backend-grpc-request")
			defer span.End()

			// --- inject trace context to metadata ─────────────────────────
			md := metadata.New(nil)
			otel.GetTextMapPropagator().Inject(ctx, telemetry.MetadataTextMapCarrier(md))
			ctx = metadata.NewOutgoingContext(ctx, md)

			// fmt.Println(md.Get("traceparent")[0])
			// fmt.Println(span.SpanContext().TraceID().String())

			fullMethod := "/simplecrud.ProductService/GetAll"
			reqMsg := &emptypb.Empty{}
			attrs := logger.LogGRPCRequest(ctx, fullMethod, md, reqMsg, "outgoing::request")
			logger.Info(ctx, "GRPC", attrs...)

			// --- call GetAll RPC with metadata ────────────────────────────
			start := time.Now()
			var trailer metadata.MD
			resp, err := client.GetAll(ctx, &emptypb.Empty{}, grpc.Trailer(&trailer))
			cancel()
			span.End()
			duration := time.Since(start)

			var grpcCode grpcCodes.Code
			if err != nil {
				if st, ok := grpcStatus.FromError(err); ok {
					grpcCode = st.Code()
				} else {
					grpcCode = grpcCodes.Unknown // fallback: 2
				}
			} else {
				grpcCode = grpcCodes.OK // success: 0
			}
			status := int32(grpcCode)

			attrs = logger.LogGRPCResponse(ctx, fullMethod, trailer, status, resp, duration, "outgoing::response")
			logger.Info(ctx, "GRPC", attrs...)

			// If the server sets the x-trace-id in the trailer, we can log it
			serverTraceID := "empty"
			if ids := trailer.Get("x-trace-id"); len(ids) > 0 {
				serverTraceID = ids[0]
			} else {
				logger.Warn(ctx, "No Trace ID received")
			}

			// --- logging response / error ────────────────────────────────────
			if err != nil {
				logger.Error(ctx, "Error calling GetAll",
					slog.String("exception.message", err.Error()),
					slog.String("exception.type", fmt.Sprintf("%T", errors.Unwrap(err))),
					slog.String("exception.stacktrace", string(debug.Stack())),
					slog.String("data.trace_id", serverTraceID),
				)
			} else {
				logger.Info(ctx, "Received products",
					slog.String("data.resolver", resp.Resolver),
					slog.Int("data.count", len(resp.GetProducts())),
					slog.String("data.trace_id", serverTraceID),
				)
			}

			delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
			time.Sleep(delay)
		}
	}
}
