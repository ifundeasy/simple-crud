package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

var (
	httpClient *http.Client
)

func init() {
	httpClient = &http.Client{
		Timeout: 5 * time.Second,
	}
}

// sendLog sends log entry in background
func sendLog(level, message string, attrs []slog.Attr) {
	go func() {
		remoteURI := os.Getenv("REMOTE_LOG_HTTP_URI")
		if remoteURI == "" {
			return
		}

		logEntry := buildLogEntry(level, message, attrs)

		jsonData, err := json.Marshal(logEntry)
		if err != nil {
			// Log error to stdout only, don't break the flow
			fmt.Fprintf(os.Stderr, "Failed to marshal for remote log entry: %v\n", err)
			return
		}

		req, err := http.NewRequest("POST", remoteURI, bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create request for remote log: %v\n", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send to remote log: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			fmt.Fprintf(os.Stderr, "Remote log returned error status: %d\n", resp.StatusCode)
		}
	}()
}
