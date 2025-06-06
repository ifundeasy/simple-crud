package config

import (
	"log/slog"
	"os"
	"simple-crud/internal/logger"
	"strconv"
	"sync"

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
	PyroscopTenantId   string
}

var log = logger.Instance()
var (
	configInstance *Config
	configOnce     sync.Once
)

func setInt64(varName string) int64 {

	val := os.Getenv(varName)
	if val == "" {
		log.Warn("Unset %s; fallback to 1s", varName)
		return 1000 // default value if not set
	}

	num, err := strconv.ParseInt(val, 10, 16)
	if err != nil {
		log.Error("Invalid %s: %v; fallback to 1s", varName, err)
		return 1000
	}

	return int64(num)
}

func Instance() *Config {
	configOnce.Do(func() {

		// Load .env file (optional)
		if err := godotenv.Load(); err != nil {
			log.Warn("No .env file found, using system environment variables")
		}

		configInstance = &Config{
			AppPort:            os.Getenv("APP_PORT"),
			AppName:            os.Getenv("APP_NAME"),
			AppClientDelayMs:   setInt64("APP_CLIENT_DELAY_MS"),
			DnsResolverDelayMs: setInt64("DNS_RESOLVER_DELAY_MS"),
			MongoURI:           os.Getenv("MONGO_URI"),
			MongoDBName:        os.Getenv("MONGO_DB_NAME"),
			ExternalGRPC:       os.Getenv("EXTERNAL_GRPC"),
			ExternalHTTP:       os.Getenv("EXTERNAL_HTTP"),
			OtelRPCURI:         os.Getenv("OTEL_RPC_URI"),
			PyroscopeURI:       os.Getenv("PYROSCOPE_URI"),
			PyroscopTenantId:   os.Getenv("PYROSCOPE_TENANTID"),
		}

		// Validate required env
		var missing []string
		if configInstance.AppPort == "" {
			missing = append(missing, "APP_PORT")
		}
		if configInstance.AppName == "" {
			missing = append(missing, "APP_NAME")
		}
		if configInstance.MongoURI == "" {
			missing = append(missing, "MONGO_URI")
		}
		if configInstance.MongoDBName == "" {
			missing = append(missing, "MONGO_DB_NAME")
		}
		if configInstance.ExternalGRPC == "" {
			missing = append(missing, "EXTERNAL_GRPC")
		}
		if configInstance.ExternalHTTP == "" {
			missing = append(missing, "EXTERNAL_HTTP")
		}
		if configInstance.OtelRPCURI == "" {
			missing = append(missing, "OTEL_RPC_URI")
		}
		if configInstance.PyroscopeURI == "" {
			missing = append(missing, "PYROSCOPE_URI")
		}

		if len(missing) > 0 {
			log.Error("Missing required environment variables", slog.Any("missing", missing))
			os.Exit(1)
		}

		log.Info("Configuration loaded successfully", slog.Any("config", configInstance))
	})

	return configInstance
}
