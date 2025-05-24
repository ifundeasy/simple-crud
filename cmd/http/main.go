package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"simple-crud/internal/config"
	"simple-crud/internal/database"
	handlerhttp "simple-crud/internal/handler/http"
	"simple-crud/internal/repository"
	"simple-crud/internal/service"
	"simple-crud/internal/telemetry"
	"simple-crud/pkg/logger"
)

func main() {
	ctx := context.Background()
	log := logger.New()

	// Load config
	cfg := config.Load(log)

	// Initialize telemetry (OpenTelemetry + Pyroscope)
	shutdown := telemetry.Init(ctx, log, cfg)
	defer shutdown()

	// Connect to MongoDB
	db, err := database.Connect(ctx, log, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		log.Error("Failed to connect to MongoDB", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Wiring
	productRepo := repository.NewProductRepository(db.Database)
	productService := service.NewProductService(productRepo)
	productHandler := handlerhttp.NewProductHandler(productService, log)

	externalHandler := handlerhttp.NewExternalHandler(cfg.ExternalHTTP, log)

	// Routing
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Info("HTTP Request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("query", r.URL.RawQuery),
			slog.String("remote", r.RemoteAddr),
			slog.String("hostname", logger.Hostname),
		)
		resp := map[string]string{"data": "hello-world"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/products", productHandler.GetAll)
	mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			productHandler.GetByID(w, r)
		case http.MethodPost:
			productHandler.Create(w, r)
		case http.MethodPut:
			productHandler.Update(w, r)
		case http.MethodDelete:
			productHandler.Delete(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/external", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			externalHandler.Fetch(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Info("HTTP server running", slog.String("addr", server.Addr))

	if err := server.ListenAndServe(); err != nil {
		log.Error("Server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
