package main

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"simple-crud/internal/config"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/pkg/logger"

	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	log := logger.New()
	cfg := config.Load(log)
	conn, err := grpc.Dial(cfg.ExternalGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Info("failed to connect to gRPC server at %s: %v", cfg.ExternalGRPC, err)
	}
	defer conn.Close()

	client := pb.NewProductServiceClient(conn)

	log.Info("gRPC client started. Connecting to %s. Calling GetAll every 5s...", cfg.ExternalGRPC)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		resp, err := client.GetAll(ctx, &emptypb.Empty{})
		cancel()

		if err != nil {
			log.Error("Error calling GetAll: %v", err)
		} else {
			log.Info("Received products",
				slog.Int("count", len(resp.GetProducts())),
			)
		}

		time.Sleep(25 * time.Millisecond)
	}
}
