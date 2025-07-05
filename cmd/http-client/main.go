package main

import (
	"context"
	"log/slog"
	"math/rand"
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

	logger.Info(globalCtx, cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
	)

	shutdown, _ := tracer.Instance(globalCtx)
	defer shutdown()

	HttpRequestorTracer := otel.Tracer("HttpRequestorMain")

	for {
		select {
		case <-globalCtx.Done():
			logger.Info(globalCtx, "Shutting down HTTP client")
			return

		default:
			ctx, cancel := context.WithTimeout(globalCtx, 2*time.Second)
			ctx, span := HttpRequestorTracer.Start(ctx, "backend-http-request")

			cfg := config.Instance()
			httpClient := client.NewHTTPClient(cfg.ExternalHTTP, 3*time.Second)

			// paths := []string{"/external", "/products", "/just-not-found"}
			paths := []string{"/external"}
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

			logger.Info(ctx, "Fetched products", slog.Int("count", len(resp.RawBody)))
			span.End()
			sleep(cfg)
		}
	}
}

func sleep(cfg *config.Config) {
	// delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
	delay := time.Duration(int(cfg.ClientMaxSleepMs)+1) * time.Millisecond
	time.Sleep(delay)
}
