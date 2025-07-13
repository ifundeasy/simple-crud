package config

import (
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unicode"

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

func toSnake(s string) string {
	var out strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			// Tambahkan underscore jika bukan huruf pertama dan sebelumnya bukan underscore
			if i > 0 && s[i-1] != '_' {
				out.WriteRune('_')
			}
			out.WriteRune(unicode.ToLower(r))
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// StructAttrs("data", cfg) ➜ []slog.Attr{ slog.String("data.app_port", "3001"), ... }
func StructAttrs(prefix string, s any) []slog.Attr {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()

	attrs := make([]slog.Attr, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		key := prefix + "." + jsonKey(f) // mis. "data.app_port"

		switch v.Field(i).Kind() {
		case reflect.String:
			attrs = append(attrs, slog.String(key, v.Field(i).String()))
		case reflect.Int, reflect.Int64, reflect.Int32:
			attrs = append(attrs, slog.Int64(key, v.Field(i).Int()))
		default:
			attrs = append(attrs, slog.Any(key, v.Field(i).Interface()))
		}
	}
	return attrs
}

// Ambil nama tag `json:"..."` kalau ada; fallback ke camelCase->snake
func jsonKey(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	return toSnake(f.Name) // implementasi toSnake sederhana
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

		attrs := StructAttrs("data", configInstance.ToSafeConfig())
		anyAttrs := make([]any, len(attrs))
		for i, a := range attrs {
			anyAttrs[i] = a
		}
		log.Info("Configuration loaded successfully", anyAttrs...)
	})

	return configInstance
}
