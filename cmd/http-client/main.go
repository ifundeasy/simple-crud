package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"log/slog"

	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"simple-crud/internal/version"
)

func main() {
	log := logger.Instance()
	cfg := config.Instance()

	log.Info(cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
	)

	if cfg.ExternalHTTP == "" {
		log.Error("EXTERNAL_HTTP environment variable is not set")
		os.Exit(1)
	}

	log.Info("HTTP client started",
		slog.String("target", cfg.ExternalHTTP),
		slog.Int("delay_ms", int(cfg.AppClientDelayMs)),
	)

	client := http.Client{Timeout: 2 * time.Second}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.ExternalHTTP+"/products", nil)
		if err != nil {
			log.Error("Failed to build request", slog.String("error", err.Error()))
			cancel()
			time.Sleep(time.Duration(cfg.AppClientDelayMs) * time.Millisecond)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Error("Failed to request", slog.String("error", err.Error()))
			cancel()
			time.Sleep(time.Duration(cfg.AppClientDelayMs) * time.Millisecond)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			log.Error("Failed to read body", slog.String("error", err.Error()))
			time.Sleep(time.Duration(cfg.AppClientDelayMs) * time.Millisecond)
			continue
		}

		var out []any
		if err := json.Unmarshal(body, &out); err != nil {
			log.Error("Invalid JSON response", slog.String("body", string(body)))
			time.Sleep(time.Duration(cfg.AppClientDelayMs) * time.Millisecond)
			continue
		}

		log.Info("Received products", slog.Int("count", len(out)))
		time.Sleep(time.Duration(cfg.AppClientDelayMs) * time.Millisecond)
	}
}
