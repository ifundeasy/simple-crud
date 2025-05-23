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
	"simple-crud/pkg/logger"
)

func main() {
	log := logger.New()
	cfg := config.Load(log)

	if cfg.ExternalHTTP == "" {
		log.Error("EXTERNAL_HTTP environment variable is not set")
		os.Exit(1)
	}

	log.Info("HTTP client started. Calling GET %s/products every 25ms...", cfg.ExternalHTTP)

	client := http.Client{Timeout: 2 * time.Second}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.ExternalHTTP+"/products", nil)
		if err != nil {
			log.Error("Failed to build request", slog.String("error", err.Error()))
			cancel()
			time.Sleep(25 * time.Millisecond)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Error("Failed to request", slog.String("error", err.Error()))
			cancel()
			time.Sleep(25 * time.Millisecond)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			log.Error("Failed to read body", slog.String("error", err.Error()))
			time.Sleep(25 * time.Millisecond)
			continue
		}

		var out []any
		if err := json.Unmarshal(body, &out); err != nil {
			log.Error("Invalid JSON response", slog.String("body", string(body)))
			time.Sleep(25 * time.Millisecond)
			continue
		}

		log.Info("Received products", slog.Int("count", len(out)))
		time.Sleep(25 * time.Millisecond)
	}
}
