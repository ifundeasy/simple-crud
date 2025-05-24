package logger

import (
	"log/slog"
	"os"
)

var Hostname string

func init() {
	h, err := os.Hostname()
	if err != nil {
		Hostname = "unknown"
	} else {
		Hostname = h
	}
}

func New() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
