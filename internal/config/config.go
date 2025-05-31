package config

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort            string
	AppName            string
	AppClientDelayMs   int64
	DnsResolverDelayMs int64
	MongoURI           string
	MongoDBName        string
	ExternalGRPC       string
	ExternalHTTP       string
	OtelRPCURI         string
	PyroscopeURI       string
}

func getClientDelay(varName string, logger *slog.Logger) int64 {
	val := os.Getenv(varName)
	if val == "" {
		logger.Warn("Unset APP_CLIENT_DELAY_MS; fallback to 1s")
		return 1000 // default value if not set
	}

	num, err := strconv.ParseInt(val, 10, 16)
	if err != nil {
		logger.Error("Invalid APP_CLIENT_DELAY_MS: %v; fallback to 1s", err)
		return 1000
	}

	return int64(num)
}

func Load(logger *slog.Logger) *Config {
	// Load .env file (optional)
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, using system environment variables")
	}

	cfg := &Config{
		AppPort:            os.Getenv("APP_PORT"),
		AppName:            os.Getenv("APP_NAME"),
		AppClientDelayMs:   getClientDelay("APP_CLIENT_DELAY_MS", logger),
		DnsResolverDelayMs: getClientDelay("DNS_RESOLVER_DELAY_MS", logger),
		MongoURI:           os.Getenv("MONGO_URI"),
		MongoDBName:        os.Getenv("MONGO_DB_NAME"),
		ExternalGRPC:       os.Getenv("EXTERNAL_GRPC"),
		ExternalHTTP:       os.Getenv("EXTERNAL_HTTP"),
		OtelRPCURI:         os.Getenv("OTEL_RPC_URI"),
		PyroscopeURI:       os.Getenv("PYROSCOPE_URI"),
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
