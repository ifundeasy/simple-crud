package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"simple-crud/internal/client"
	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"simple-crud/internal/tracer"
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

	logger.Info(globalCtx, cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
		slog.Bool("gracefulShutdown", isProduction),
	)

	shutdown, _ := tracer.Instance(globalCtx)
	defer shutdown()

	HttpRequestorTracer := otel.Tracer("HttpRequestorMain")

	for {
		select {
		case <-globalCtx.Done():
			if !isProduction {
				logger.Info(globalCtx, "Received shutdown signal, exiting immediately")
				os.Exit(0)
			} else {
				logger.Info(globalCtx, "Shutting down HTTP client")
				return
			}

		default:
			ctx, cancel := context.WithTimeout(globalCtx, 2*time.Second)
			ctx, span := HttpRequestorTracer.Start(ctx, "backend-http-request")

			cfg := config.Instance()
			httpClient := client.NewHTTPClient(cfg.ExternalHTTP, 3*time.Second)

			paths := []string{"/external", "/products", "/just-not-found"}
			path := paths[rand.Intn(len(paths))]
			resp, err := httpClient.GetWithResponse(path, client.RequestOptions{
				Context: ctx,
			})
			if err != nil {
				cancel()
				span.End()
				sleep(cfg)
				continue
			}

			var products []interface{}
			if err := json.Unmarshal(resp.RawBody, &products); err == nil {
				logger.Info(ctx, "Fetched products", slog.Int("count", len(products)))
			} else {
				logger.Error(ctx, "Failed to parse JSON response", slog.String("error", err.Error()))
			}
			span.End()
			sleep(cfg)
		}
	}
}

func sleep(cfg *config.Config) {
	delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
	time.Sleep(delay)
}
