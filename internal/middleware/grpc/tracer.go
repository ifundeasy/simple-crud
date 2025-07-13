package middleware_grpc

import (
	"context"
	"time"

	"simple-crud/internal/logger"
	"simple-crud/internal/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	grpcStatus "google.golang.org/grpc/status"
)

var tracer = otel.Tracer("GrpcMiddleware")

// UnaryTracingInterceptor returns a gRPC unary server interceptor that
// 1) propagates/creates spans,
// 2) logs request & response via logger.LogGRPCRequest/Response,
// 3) injects X-Trace-ID trailer, and
// 4) handles panic recovery consistently.
func UnaryTracingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		// Extract existing tracing headers
		md, _ := metadata.FromIncomingContext(ctx)
		carrier := telemetry.MetadataTextMapCarrier(md)
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		ctx, span := tracer.Start(ctx, info.FullMethod)
		start := time.Now()

		defer func() {
			if rec := recover(); rec != nil {
				span.RecordError(errFromRecover(rec))
				span.SetStatus(codes.Error, "panic occurred")
				panic(rec)
			}
			span.End()
		}()

		// Remote address (for logging only)
		var remoteAddr string
		if p, ok := peer.FromContext(ctx); ok {
			remoteAddr = p.Addr.String()
		}

		// ---- Log incoming request -------------------------------------
		reqAttrs := logger.LogGRPCRequest(ctx, info.FullMethod, md, req, "incoming::request")
		logger.Info(ctx, "GRPC", reqAttrs...)

		// Call handler
		resp, err = handler(ctx, req)
		duration := time.Since(start)

		// ---- Post‑processing span status ------------------------------
		if err != nil {
			span.RecordError(err)
			if st, ok := grpcStatus.FromError(err); ok {
				span.SetAttributes(attribute.String("grpc.status", st.Code().String()))
			}
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}

		// ---- Prepare trailer + trace ID -------------------------------
		traceID := span.SpanContext().TraceID().String()
		trailerMD := metadata.Pairs("x-trace-id", traceID)
		grpc.SetTrailer(ctx, trailerMD)

		span.SetAttributes(attribute.String("grpc.remote_addr", remoteAddr))

		// ---- Log outgoing response ------------------------------------
		var grpcCode grpcCodes.Code

		if err != nil {
			if st, ok := grpcStatus.FromError(err); ok {
				grpcCode = st.Code()
			} else {
				grpcCode = grpcCodes.Unknown // fallback: 2
			}
		} else {
			grpcCode = grpcCodes.OK // success: 0
		}
		status := int32(grpcCode)

		respAttrs := logger.LogGRPCResponse(
			ctx,
			info.FullMethod,
			trailerMD,
			status,
			resp,
			duration,
			"incoming::response",
		)

		logger.Info(ctx, "GRPC", respAttrs...)

		return resp, err

	}
}

// Panic recovery (biar seragam sama HTTP middleware‑mu)
func errFromRecover(rec interface{}) error {
	if err, ok := rec.(error); ok {
		return err
	}
	return &panicError{rec}
}

type panicError struct{ value interface{} }

func (p *panicError) Error() string { return "panic: " + stringify(p.value) }

func stringify(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "unknown panic"
}
