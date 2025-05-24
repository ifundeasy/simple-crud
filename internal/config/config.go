package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort      string
	AppName      string
	MongoURI     string
	MongoDBName  string
	ExternalGRPC string
	ExternalHTTP string
	OtelRPCURI   string
	PyroscopeURI string
}

func Load(logger *slog.Logger) *Config {
	// Load .env file (optional)
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, using system environment variables")
	}

	cfg := &Config{
		AppPort:      os.Getenv("APP_PORT"),
		AppName:      os.Getenv("APP_NAME"),
		MongoURI:     os.Getenv("MONGO_URI"),
		MongoDBName:  os.Getenv("MONGO_DB_NAME"),
		ExternalGRPC: os.Getenv("EXTERNAL_GRPC"),
		ExternalHTTP: os.Getenv("EXTERNAL_HTTP"),
		OtelRPCURI:   os.Getenv("OTEL_RPC_URI"),
		PyroscopeURI: os.Getenv("PYROSCOPE_URI"),
	}

	// Validate required env
	var missing []string
	if cfg.AppPort == "" {
		missing = append(missing, "APP_PORT")
	}
	if cfg.AppName == "" {
		missing = append(missing, "APP_NAME")
	}
	if cfg.MongoURI == "" {
		missing = append(missing, "MONGO_URI")
	}
	if cfg.MongoDBName == "" {
		missing = append(missing, "MONGO_DB_NAME")
	}
	if cfg.ExternalHTTP == "" {
		missing = append(missing, "EXTERNAL_GRPC")
	}
	if cfg.ExternalHTTP == "" {
		missing = append(missing, "EXTERNAL_API")
	}
	if cfg.OtelRPCURI == "" {
		missing = append(missing, "OTEL_RPC_URI")
	}
	if cfg.PyroscopeURI == "" {
		missing = append(missing, "PYROSCOPE_URI")
	}

	if len(missing) > 0 {
		logger.Error("Missing required environment variables", slog.Any("missing", missing))
		os.Exit(1)
	}

	logger.Info("Configuration loaded successfully", slog.Any("config", cfg))
	return cfg
}

func (c *Config) GetAppName() string {
	return c.AppName
}

func (c *Config) GetOtelRPCURI() string {
	return c.OtelRPCURI
}

func (c *Config) GetPyroscopeURI() string {
	return c.PyroscopeURI
}
