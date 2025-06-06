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
	handler "simple-crud/internal/handler/http"
	"simple-crud/internal/logger"
	middleware_http "simple-crud/internal/middleware/http"
	"simple-crud/internal/repository"
	"simple-crud/internal/service"
	"simple-crud/internal/tracer"
	"simple-crud/internal/version"
)

func main() {
	globalCtx := context.Background()
	log := logger.Instance()
	cfg := config.Instance()

	log.Info(cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
	)

	// Initialize telemetry (OpenTelemetry + Pyroscope)
	shutdown, _ := tracer.Instance(globalCtx)
	defer shutdown()

	// Connect to MongoDB
	db, err := database.Instance(globalCtx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		log.Error("Failed to connect to MongoDB", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Wiring
	productRepo := repository.NewProductRepository(db.Database)
	productService := service.NewProductService(productRepo)
	productHandler := handler.NewProductHandler(productService)
	externalHandler := handler.NewExternalHandler(cfg.ExternalHTTP)

	// Wiring health service
	healthService := service.NewHealthService(db.Client)
	healthHandler := handler.NewHealthHandler(healthService)

	// Routing
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]string{"data": "hello-world"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		productHandler.GetAll(globalCtx, w, r)
	})

	mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			productHandler.GetByID(globalCtx, w, r)
		case http.MethodPost:
			productHandler.Create(globalCtx, w, r)
		case http.MethodPut:
			productHandler.Update(globalCtx, w, r)
		case http.MethodDelete:
			productHandler.Delete(globalCtx, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/external", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			externalHandler.Fetch(globalCtx, w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			healthHandler.Check(globalCtx, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// HTTP server
	wrappedMux := middleware_http.TraceMiddleware(globalCtx)(mux)
	server := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      wrappedMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Info("HTTP server running", slog.String("addr", server.Addr))

	if err := server.ListenAndServe(); err != nil {
		log.Error("Server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
