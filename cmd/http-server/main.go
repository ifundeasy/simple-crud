package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	// Create cancellable context for graceful shutdown
	bgCtx := context.Background()
	globalCtx, cancel := signal.NotifyContext(bgCtx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Instance()
	cfg := config.Instance()

	logger.Info(globalCtx, cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
	)

	// Initialize telemetry
	shutdown, _ := tracer.Instance(globalCtx)
	defer shutdown()

	// Connect to MongoDB
	db, err := database.Instance(globalCtx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		logger.Error(globalCtx, "Failed to connect to MongoDB", slog.String("error", err.Error()))
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
		productHandler.GetAll(w, r)
	})

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

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			healthHandler.Check(w, r)
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

	go func() {
		logger.Info(globalCtx, "HTTP server running", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(globalCtx, "Server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-globalCtx.Done() // wait for interrupt
	logger.Info(globalCtx, "Shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error(globalCtx, "Graceful shutdown failed", slog.String("error", err.Error()))
	} else {
		logger.Info(globalCtx, "Server exited properly")
	}
}
