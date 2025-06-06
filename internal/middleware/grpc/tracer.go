package middleware_grpc

import (
	"context"
	"encoding/json"
	"log/slog"

	"simple-crud/internal/logger"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

var tracer = otel.Tracer("GrpcMiddleware")

func UnaryTracingInterceptor(globalCtx context.Context) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {

		ctx, span := tracer.Start(ctx, info.FullMethod)
		defer func() {
			if rec := recover(); rec != nil {
				span.RecordError(errFromRecover(rec))
				span.SetStatus(codes.Error, "panic occurred")
				panic(rec)
			}
			span.End()
		}()

		// Extract remote address
		var remoteAddr string
		if p, ok := peer.FromContext(ctx); ok {
			remoteAddr = p.Addr.String()
		}

		// Extract request body
		reqBody, _ := json.Marshal(req)

		// Extract trace ID for enrichment
		traceID := span.SpanContext().TraceID().String()

		// Inject trace ID into gRPC response trailer
		grpc.SetTrailer(ctx, metadata.Pairs("x-trace-id", traceID))

		// Call actual gRPC handler
		resp, err = handler(ctx, req)

		// Set span attributes after handler completes
		span.SetAttributes(
			attribute.String("grpc.method", info.FullMethod),
			attribute.String("grpc.remote_addr", remoteAddr),
			attribute.String("grpc.trace_id", traceID),
		)

		if err != nil {
			span.RecordError(err)

			st, ok := status.FromError(err)
			if ok {
				span.SetAttributes(attribute.String("grpc.code", st.Code().String()))
			}

			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		logger.Info(ctx, "GrpcMiddleware",
			slog.String("trace_id", traceID),
			slog.String("grpc.method", info.FullMethod),
			slog.String("grpc.remote", remoteAddr),
			slog.String("grpc.body", string(reqBody)),
		)

		return resp, err
	}
}

// Panic recovery (biar seragam sama HTTP middleware-mu)
func errFromRecover(rec interface{}) error {
	if err, ok := rec.(error); ok {
		return err
	}
	return &panicError{rec}
}

type panicError struct {
	value interface{}
}

func (p *panicError) Error() string {
	return "panic: " + stringify(p.value)
}

func stringify(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	default:
		return "unknown panic"
	}
}
