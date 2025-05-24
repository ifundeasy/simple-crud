package telemetry

import (
	"context"
	"log/slog"
	"os"

	"github.com/grafana/pyroscope-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type Config interface {
	GetAppName() string
	GetOtelRPCURI() string
	GetPyroscopeURI() string
}

type AppConfig struct {
	AppName      string
	OtelRPCURI   string
	PyroscopeURI string
}

func (a AppConfig) GetAppName() string      { return a.AppName }
func (a AppConfig) GetOtelRPCURI() string   { return a.OtelRPCURI }
func (a AppConfig) GetPyroscopeURI() string { return a.PyroscopeURI }

func Init(ctx context.Context, log *slog.Logger, cfg Config) func() {
	appName := cfg.GetAppName()

	// Start Pyroscope
	_, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: appName,
		ServerAddress:   cfg.GetPyroscopeURI(),
		Logger:          pyroscope.StandardLogger,
	})
	if err != nil {
		log.Error("Pyroscope failed to start", slog.String("error", err.Error()))
	} else {
		log.Info("Pyroscope started successfully")
	}

	// OTLP exporter
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(cfg.GetOtelRPCURI()),
		otlptracegrpc.WithCompressor("gzip"),
	)
	if err != nil {
		log.Error("Failed to create OTLP exporter", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Tracer provider
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(appName),
			attribute.String("env", "production"),
		),
	)
	if err != nil {
		log.Error("Failed to create resource", slog.String("error", err.Error()))
		os.Exit(1)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	log.Info("OpenTelemetry Tracer initialized")

	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Error("Error shutting down tracer provider", slog.String("error", err.Error()))
		}
	}
}
