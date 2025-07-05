package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"simple-crud/internal/config"
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
	tracer := otel.Tracer("http-client")

	logger.Info(globalCtx, cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
	)

	client := http.Client{Timeout: 2 * time.Second}

	for {
		select {
		case <-globalCtx.Done():
			logger.Info(globalCtx, "Shutting down HTTP client")
			return

		default:
			ctx, cancel := context.WithTimeout(globalCtx, 2*time.Second)
			ctx, span := tracer.Start(ctx, "http-request")

			paths := []string{"/external", "/products", "/just-not-found"}
			path := paths[rand.Intn(len(paths))]
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.ExternalHTTP+path, nil)
			if err != nil {
				logger.Error(ctx, "Failed to build request", slog.String("error", err.Error()))
				cancel()
				span.End()
				sleep(cfg)
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				logger.Error(ctx, "Failed to request", slog.String("error", err.Error()))
				cancel()
				span.End()
				sleep(cfg)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			cancel()
			if err != nil {
				logger.Error(ctx, "Failed to read body", slog.String("error", err.Error()))
				span.End()
				sleep(cfg)
				continue
			}

			var out []any
			if err := json.Unmarshal(body, &out); err != nil {
				logger.Error(ctx, "Invalid JSON response", slog.String("body", string(body)))
				span.End()
				sleep(cfg)
				continue
			}

			logger.Info(ctx, "Received products", slog.Int("count", len(out)))
			span.End()
			sleep(cfg)
		}
	}
}

func sleep(cfg *config.Config) {
	delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
	time.Sleep(delay)
}
