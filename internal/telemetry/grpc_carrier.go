package telemetry

import "google.golang.org/grpc/metadata"

// metadataTextMapCarrier adapts gRPC metadata.MD to OpenTelemetry TextMapCarrier
type MetadataTextMapCarrier metadata.MD

func (c MetadataTextMapCarrier) Get(key string) string {
	v := metadata.MD(c).Get(key)
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
func (c MetadataTextMapCarrier) Set(key string, value string) {
	metadata.MD(c).Set(key, value)
}
func (c MetadataTextMapCarrier) Keys() []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	return out
}
