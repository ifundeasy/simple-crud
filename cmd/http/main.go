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
	"simple-crud/internal/repository"
	"simple-crud/internal/service"
	"simple-crud/internal/tracer"
	"simple-crud/internal/utils"
	"simple-crud/internal/version"

	"go.opentelemetry.io/otel"
)

func main() {
	ctx := context.Background()
	log := logger.Instance()
	cfg := config.Instance()

	log.Info(cfg.AppName,
		slog.String("version", version.Version),
		slog.String("commit", version.Commit),
		slog.String("buildTime", version.BuildTime),
	)

	// Initialize telemetry (OpenTelemetry + Pyroscope)
	shutdown, _ := tracer.Instance(ctx)
	defer shutdown()

	// Connect to MongoDB
	db, err := database.Instance(ctx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		log.Error("Failed to connect to MongoDB", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Wiring
	productRepo := repository.NewProductRepository(db.Database)
	productService := service.NewProductService(productRepo)
	productHandler := handler.NewProductHandler(productService)
	externalHandler := handler.NewExternalHandler(cfg.ExternalHTTP)

	// Routing
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Info("HTTP Request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("query", r.URL.RawQuery),
			slog.String("remote", r.RemoteAddr),
			slog.String("hostname", utils.GetHost()),
		)
		resp := map[string]string{"data": "hello-world"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer(cfg.AppName)
		ctx, span := tracer.Start(r.Context(), "handlerProducts")
		defer span.End()

		productHandler.GetAll(ctx, w, r)
	})

	mux.HandleFunc("/product", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer(cfg.AppName)
		ctx, span := tracer.Start(r.Context(), "handlerProduct")
		defer span.End()

		switch r.Method {
		case http.MethodGet:
			productHandler.GetByID(ctx, w, r)
		case http.MethodPost:
			productHandler.Create(ctx, w, r)
		case http.MethodPut:
			productHandler.Update(ctx, w, r)
		case http.MethodDelete:
			productHandler.Delete(ctx, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/external", func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer(cfg.AppName)
		ctx, span := tracer.Start(r.Context(), "handlerExternal")
		defer span.End()

		if r.Method == http.MethodGet {
			externalHandler.Fetch(ctx, w, r)
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
