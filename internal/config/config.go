package config

import (
	"log/slog"
	"os"
	"strconv"
	"sync"

	"simple-crud/internal/logger"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort                string
	AppName                string
	ClientMaxSleepMs       int64
	DnsResolverDelayMs     int64
	MongoURI               string
	MongoDBName            string
	ExternalGRPC           string
	ExternalHTTP           string
	RemoteLogHttpURI       string
	RemoteTraceRpcURI      string
	RemoteProfilingHttpURI string
}

// SafeConfig adalah struct untuk logging yang aman (tanpa sensitive data)
type SafeConfig struct {
	AppPort                string `json:"app_port"`
	AppName                string `json:"app_name"`
	ClientMaxSleepMs       int64  `json:"client_max_sleep_ms"`
	DnsResolverDelayMs     int64  `json:"dns_resolver_delay_ms"`
	MongoDBName            string `json:"mongo_db_name"`
	ExternalGRPC           string `json:"external_grpc"`
	ExternalHTTP           string `json:"external_http"`
	RemoteLogHttpURI       string `json:"remote_log_http_uri"`
	RemoteTraceRpcURI      string `json:"remote_trace_rpc_uri"`
	RemoteProfilingHttpURI string `json:"remote_profiling_http_uri"`
}

// ToSafeConfig mengkonversi Config ke SafeConfig untuk logging
func (c *Config) ToSafeConfig() SafeConfig {
	return SafeConfig{
		AppPort:                c.AppPort,
		AppName:                c.AppName,
		ClientMaxSleepMs:       c.ClientMaxSleepMs,
		DnsResolverDelayMs:     c.DnsResolverDelayMs,
		MongoDBName:            c.MongoDBName,
		ExternalGRPC:           c.ExternalGRPC,
		ExternalHTTP:           c.ExternalHTTP,
		RemoteLogHttpURI:       c.RemoteLogHttpURI,
		RemoteTraceRpcURI:      c.RemoteTraceRpcURI,
		RemoteProfilingHttpURI: c.RemoteProfilingHttpURI,
	}
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
			AppPort:                os.Getenv("APP_PORT"),
			AppName:                os.Getenv("APP_NAME"),
			ClientMaxSleepMs:       setInt64("CLIENT_MAX_SLEEP_MS"),
			DnsResolverDelayMs:     setInt64("DNS_RESOLVER_DELAY_MS"),
			MongoURI:               os.Getenv("MONGO_URI"),
			MongoDBName:            os.Getenv("MONGO_DB_NAME"),
			ExternalGRPC:           os.Getenv("EXTERNAL_GRPC"),
			ExternalHTTP:           os.Getenv("EXTERNAL_HTTP"),
			RemoteLogHttpURI:       os.Getenv("REMOTE_LOG_HTTP_URI"),
			RemoteTraceRpcURI:      os.Getenv("REMOTE_TRACE_RPC_URI"),
			RemoteProfilingHttpURI: os.Getenv("REMOTE_PROFILING_HTTP_URI"),
		}

		// Optional but recommended
		if configInstance.RemoteLogHttpURI == "" {
			log.Warn("Missing REMOTE_LOG_HTTP_URI will skip sending log")
		}
		if configInstance.RemoteTraceRpcURI == "" {
			log.Warn("Missing REMOTE_TRACE_RPC_URI will skip sending trace")
		}
		if configInstance.RemoteProfilingHttpURI == "" {
			log.Warn("Missing REMOTE_PROFILING_HTTP_URI will skip sending profiling")
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

		if len(missing) > 0 {
			log.Error("Missing required environment variables", slog.Any("missing", missing))
			os.Exit(1)
		}

		log.Info("Configuration loaded successfully", slog.Any("config", configInstance.ToSafeConfig()))
	})

	return configInstance
}
