package middleware_grpc

import (
	"context"
	"encoding/json"
	"log/slog"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"simple-crud/internal/logger"
)

var tracer = otel.Tracer("GrpcMiddleware")

func UnaryTracingInterceptor(globalCtx context.Context) grpc.UnaryServerInterceptor {

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {

		// Start span with gRPC full method name as operation name
		ctx, span := tracer.Start(ctx, info.FullMethod)
		defer span.End()

		var remoteAddr string
		if p, ok := peer.FromContext(ctx); ok {
			remoteAddr = p.Addr.String()
		}

		reqBody, _ := json.Marshal(req)

		// Log request entry (once per gRPC call)
		logger.Info(ctx, "GrpcMiddleware",
			slog.String("grpc.method", info.FullMethod),
			slog.String("grpc.remote", remoteAddr),
			slog.String("grpc.body", string(reqBody)),
		)

		// Continue to handler
		return handler(ctx, req)
	}
}
