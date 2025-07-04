package main

import (
	"context"
	"encoding/json"
	"io"
	"math/rand"
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
		slog.Int("delay_ms", int(cfg.ClientMaxSleepMs)),
	)

	client := http.Client{Timeout: 2 * time.Second}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		paths := []string{"/external", "/products", "/just-not-found"}
		path := paths[rand.Intn(len(paths))]
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.ExternalHTTP+path, nil)
		if err != nil {
			log.Error("Failed to build request",
				slog.String("error", err.Error()),
				slog.String("trace_id", "empty"),
			)
			cancel()
			delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
			time.Sleep(delay)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Error("Failed to request",
				slog.String("error", err.Error()),
				slog.String("trace_id", "empty"),
			)
			cancel()
			delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
			time.Sleep(delay)
			continue
		}

		// Extract trace ID from response header
		traceID := resp.Header.Get("X-TRACE-ID")
		if traceID == "" {
			traceID = "empty"
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			log.Error("Failed to read body",
				slog.String("error", err.Error()),
				slog.String("trace_id", traceID),
			)
			delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
			time.Sleep(delay)
			continue
		}

		var out []any
		if err := json.Unmarshal(body, &out); err != nil {
			log.Error("Invalid JSON response",
				slog.String("body", string(body)),
				slog.String("trace_id", traceID),
			)
			delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
			time.Sleep(delay)
			continue
		}

		log.Info("Received products",
			slog.Int("count", len(out)),
			slog.String("trace_id", traceID),
		)

		delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
		time.Sleep(delay)
	}
}
