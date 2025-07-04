package main

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"simple-crud/internal/config"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/logger"
	"simple-crud/internal/version"

	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	log := logger.Instance()
	cfg := config.Instance()

	log.Info(cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
	)

	conn, err := grpc.NewClient(
		cfg.ExternalGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)

	if err != nil {
		log.Info("failed to connect to gRPC server at %s: %v", cfg.ExternalGRPC, err)
	}
	defer conn.Close()

	client := pb.NewProductServiceClient(conn)

	log.Info("gRPC client started",
		slog.String("target", cfg.ExternalGRPC),
		slog.Int("delay_ms", int(cfg.ClientMaxSleepMs)),
	)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		var trailer metadata.MD
		resp, err := client.GetAll(ctx, &emptypb.Empty{}, grpc.Trailer(&trailer))
		cancel()

		// Extract trace-id from trailer
		traceIDs := trailer.Get("x-trace-id")
		var traceID string
		if len(traceIDs) < 1 {
			traceID = "empty"
			log.Warn("No Trace ID received")
		} else {
			traceID = traceIDs[0]
		}

		if err != nil {
			log.Error("Error calling GetAll", slog.String("error", err.Error()), slog.String("trace_id", traceID))
		} else {
			log.Info("Received products",
				slog.String("resolver", resp.Resolver),
				slog.String("trace_id", traceID),
				slog.Int("count", len(resp.GetProducts())),
			)
		}

		time.Sleep(time.Duration(cfg.ClientMaxSleepMs) * time.Millisecond)
	}
}
