package tracer

import (
	"context"
	"log/slog"
	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"sync"

	"github.com/grafana/pyroscope-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var (
	once         sync.Once
	shutdownFunc func()
	initErr      error
)

// Singleton Instance
func Instance(globalCtx context.Context) (func(), error) {
	once.Do(func() {
		cfg := config.Instance()

		// Start Pyroscope
		_, err := pyroscope.Start(pyroscope.Config{
			ApplicationName: cfg.AppName,
			ServerAddress:   cfg.PyroscopeURI,
			Logger:          pyroscope.StandardLogger,
		})

		log := logger.Instance()
		if err != nil {
			log.Error("Pyroscope failed to start", slog.String("error", err.Error()))
		} else {
			log.Info("Pyroscope started successfully")
		}

		// OTLP exporter
		exp, err := otlptracegrpc.New(globalCtx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(cfg.OtelRPCURI),
			otlptracegrpc.WithCompressor("gzip"),
		)
		if err != nil {
			log.Error("Failed to create OTLP exporter", slog.String("error", err.Error()))
			initErr = err
			return
		}

		// Tracer provider
		res, err := resource.New(globalCtx,
			resource.WithAttributes(
				semconv.ServiceNameKey.String(cfg.AppName),
				attribute.String("env", "production"),
			),
		)
		if err != nil {
			log.Error("Failed to create resource", slog.String("error", err.Error()))
			initErr = err
			return
		}

		tp := trace.NewTracerProvider(
			trace.WithBatcher(exp),
			trace.WithResource(res),
		)
		otel.SetTracerProvider(tp)

		log.Info("OpenTelemetry Tracer initialized")

		shutdownFunc = func() {
			if err := tp.Shutdown(globalCtx); err != nil {
				log.Error("Error shutting down tracer provider", slog.String("error", err.Error()))
			}
		}
	})

	return shutdownFunc, initErr
}
