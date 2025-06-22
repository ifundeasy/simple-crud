package logger

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// buildLogEntry creates a log entry compatible with Alloy/Loki format
func buildLogEntry(level, message string, attrs []slog.Attr) map[string]interface{} {
	entry := map[string]interface{}{
		"streams": []map[string]interface{}{
			{
				"stream": map[string]string{
					"level": level,
					"job":   "simple-crud",
				},
				"values": [][]string{
					{
						fmt.Sprintf("%d", time.Now().UnixNano()),
						buildLogLine(level, message, attrs),
					},
				},
			},
		},
	}

	return entry
}

// buildLogLine creates the actual log line with all attributes
func buildLogLine(level, message string, attrs []slog.Attr) string {
	logData := map[string]interface{}{
		"level":   level,
		"message": message,
		"time":    time.Now().Format(time.RFC3339),
	}

	// Add all attributes to the log data
	for _, attr := range attrs {
		logData[attr.Key] = attr.Value.Any()
	}

	jsonBytes, _ := json.Marshal(logData)
	return string(jsonBytes)
}
