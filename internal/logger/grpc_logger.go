package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ---- Metadata flattening ------------------------------------------------

var allowedMD = map[string]bool{
	"content-type":  true,
	"user-agent":    true,
	"x-trace-id":    true,
	"traceparent":   true,
	"authorization": true, // will be redacted
}

// MetadataAttrs converts gRPC metadata into []slog.Attr (grpc.header.*).
func MetadataAttrs(md metadata.MD) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(md))
	for k, vs := range md {
		lower := strings.ToLower(k)
		if !allowedMD[lower] {
			continue
		}
		v := strings.Join(vs, ", ")
		if lower == "authorization" {
			v = "***"
		}
		attrs = append(attrs, slog.String("grpc.header."+lower, v))
	}
	return attrs
}

// ---- Message flattening -------------------------------------------------

// msgAttrs flattens a protobuf message (or any value) into slog.Attr slice with prefix.
func msgAttrs(prefix string, m interface{}) []slog.Attr {
	if m == nil {
		return nil
	}
	// If proto message – turn into JSON then reuse jsonAttrs from http_logger.go.
	if pm, ok := m.(proto.Message); ok {
		b, err := protojson.Marshal(pm)
		if err == nil {
			a, _ := jsonAttrsWithPrefix(prefix, b)
			return a
		}
	}
	// Fallback: stringify.
	return []slog.Attr{slog.String(prefix, redactIfNeeded(fmt.Sprintf("%v", m)))}
}

// jsonAttrsWithPrefix reuses existing flattenJSON logic but with custom prefix.
func jsonAttrsWithPrefix(prefix string, b []byte) ([]slog.Attr, error) {
	var data any
	if err := json.Unmarshal(b, &data); err != nil {
		return []slog.Attr{slog.String(prefix, string(b))}, nil
	}
	attrs := make([]slog.Attr, 0, 8)
	flattenJSON(prefix, data, &attrs)
	return attrs, nil
}

// ---- Public helpers -----------------------------------------------------

// LogGRPCRequest builds slog attributes for an outgoing or incoming gRPC request.
// fullMethod = "/package.Service/Method" per gRPC spec.
func LogGRPCRequest(ctx context.Context, fullMethod string, md metadata.MD, req interface{}, direction string) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("grpc.direction", direction),
		slog.String("grpc.method", fullMethod),
	}
	attrs = append(attrs, MetadataAttrs(md)...)
	attrs = append(attrs, msgAttrs("grpc.request", req)...)
	return attrs
}

// LogGRPCResponse builds slog attributes for a gRPC response.
func LogGRPCResponse(ctx context.Context, fullMethod string, md metadata.MD, code codes.Code, resp interface{}, duration time.Duration, direction string) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("grpc.direction", direction),
		slog.String("grpc.method", fullMethod),
		slog.String("grpc.code", code.String()),
		slog.Int64("grpc.duration_ms", duration.Milliseconds()),
	}
	attrs = append(attrs, MetadataAttrs(md)...)
	attrs = append(attrs, msgAttrs("grpc.response", resp)...)
	return attrs
}
