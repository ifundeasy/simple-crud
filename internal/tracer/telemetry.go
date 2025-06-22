package tracer

import (
	"context"
	"log/slog"
	"simple-crud/internal/config"
	"simple-crud/internal/logger"
	"sync"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	"github.com/grafana/pyroscope-go"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var (
	once         sync.Once
	shutdownFunc func()
	initErr      error
)

var pyroLogrus = func() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.InfoLevel)
	return l
}()

// Singleton Instance
func Instance(globalCtx context.Context) (func(), error) {
	once.Do(func() {
		cfg := config.Instance()
		log := logger.Instance()

		// OTLP exporter (Tempo, etc)
		exp, err := otlptracegrpc.New(globalCtx,
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(cfg.RemoteTraceRpcURI),
			otlptracegrpc.WithCompressor("gzip"),
		)
		if err != nil {
			log.Error("Failed to create OTLP exporter", slog.String("error", err.Error()))
			initErr = err
			return
		}

		// OpenTelemetry Resource (service name, env, etc)
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

		// Set tracer provider WITH pyroscope attached
		otel.SetTracerProvider(otelpyroscope.NewTracerProvider(tp))

		// Register the trace context and baggage propagators so data is propagated across services/processes.
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

		log.Info("OpenTelemetry Tracer initialized")

		// Start Pyroscope profiler agent
		_, err2 := pyroscope.Start(pyroscope.Config{
			ApplicationName: cfg.AppName,
			ServerAddress:   cfg.RemoteProfilingHttpURI,
			// TenantID:        cfg.RemoteProfilingTenantId,
			// Logger:          pyroscope.StandardLogger,
			Logger: pyroLogrus,
		})
		if err2 != nil {
			log.Error("Pyroscope failed to start", slog.String("error", err2.Error()))
		} else {
			log.Info("Pyroscope started successfully")
		}

		shutdownFunc = func() {
			if err := tp.Shutdown(globalCtx); err != nil {
				log.Error("Error shutting down tracer provider", slog.String("error", err.Error()))
			}
		}
	})

	return shutdownFunc, initErr
}
