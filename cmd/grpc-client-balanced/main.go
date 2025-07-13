package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"simple-crud/internal/config"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/logger"
	"simple-crud/internal/telemetry"
	"simple-crud/internal/version"

	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	conn   *grpc.ClientConn
	client pb.ProductServiceClient
	cfg    = config.Instance()
)

// hashAddresses generates a SHA256 hash from list of resolved pod IP addresses.
// We use this hash to detect pod scaling or IP changes.
func hashAddresses(addresses []string) string {
	j, _ := json.Marshal(addresses)
	h := sha256.Sum256(j)
	return string(h[:])
}

// resolvePods performs DNS lookup on the headless service hostname.
// This uses Go's default net.Resolver (which uses /etc/resolv.conf inside Kubernetes).
func resolvePods(host string) ([]string, error) {
	return net.LookupHost(host)
}

// connect establishes gRPC client connection to the backend server using round_robin policy.
func connect(globalCtx context.Context, target string) error {
	var err error
	conn, err = grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		logger.Error(globalCtx, "Failed to connect to gRPC server",
			slog.String("grpc.remote_addr", cfg.ExternalGRPC),
			slog.String("exception.message", err.Error()),
			slog.String("exception.type", fmt.Sprintf("%T", errors.Unwrap(err))),
			slog.String("exception.stacktrace", string(debug.Stack())),
		)
		return err
	}
	client = pb.NewProductServiceClient(conn)
	logger.Info(globalCtx, "gRPC connected", slog.String("target", target))
	return nil
}

func disconnect(globalCtx context.Context) {
	if conn != nil {
		conn.Close()
		logger.Info(globalCtx, "gRPC connection closed")
	}
}

// dnsWatcher periodically resolves DNS and notifies the gRPC worker if any changes detected.
func dnsWatcher(globalCtx context.Context, host string, notify chan struct{}) {
	var lastHash string
	var consecutiveFail int
	var cleanHost string = host
	if strings.HasPrefix(cleanHost, "dns://") {
		cleanHost = strings.TrimPrefix(cleanHost, "dns://")
		cleanHost = strings.TrimLeft(cleanHost, "/")
	}
	// Extract hostname (without port and protocol) for DNS resolution
	cleanHost = strings.Split(cleanHost, ":")[0]

	for {
		select {
		case <-globalCtx.Done():
			logger.Info(globalCtx, "DNS watcher shutting down")
			return
		default:
		}

		logger.Info(globalCtx, "Just keep watching gRPC DNS..", slog.Any("host", cleanHost))
		addrs, err := resolvePods(cleanHost)
		if err != nil {
			// DNS lookup failed
			logger.Error(globalCtx, "DNS lookup failed",
				slog.String("exception.message", err.Error()),
				slog.String("exception.type", fmt.Sprintf("%T", errors.Unwrap(err))),
				slog.String("exception.stacktrace", string(debug.Stack())),
				slog.Any("data", cleanHost),
			)
			consecutiveFail++

			// Go's built-in net.LookupHost() does not implement any DNS retry, failover, or backoff strategy.
			if consecutiveFail >= 3 {
				logger.Warn(globalCtx, "DNS lookup failed 3 times consecutively, forcing reconnect", slog.Any("host", cleanHost))
				notify <- struct{}{}
				consecutiveFail = 0
			}
			time.Sleep(time.Duration(cfg.DnsResolverDelayMs) * time.Millisecond)
			continue
		}

		consecutiveFail = 0
		sort.Strings(addrs)
		newHash := hashAddresses(addrs)
		if newHash != lastHash {
			logger.Info(globalCtx, "Detected backend pod change", slog.Any("host", cleanHost), slog.Any("addresses", addrs))
			lastHash = newHash
			notify <- struct{}{}
		}
		time.Sleep(time.Duration(cfg.DnsResolverDelayMs) * time.Millisecond)
	}
}

// grpcWorker performs gRPC request loop and reconnects whenever DNS watcher sends signal.
func grpcWorker(globalCtx context.Context, notify chan struct{}) {
	tracer := otel.Tracer("backend-grpc-client")
	defer disconnect(globalCtx)
	err := connect(globalCtx, cfg.ExternalGRPC)
	if err != nil {
		logger.Error(globalCtx, "Initial gRPC connection failed, will retry on next DNS change",
			slog.String("grpc.remote_addr", cfg.ExternalGRPC),
			slog.String("exception.message", err.Error()),
			slog.String("exception.type", fmt.Sprintf("%T", errors.Unwrap(err))),
			slog.String("exception.stacktrace", string(debug.Stack())),
		)
	}

	for {
		select {
		case <-globalCtx.Done():
			logger.Info(globalCtx, "Shutting down gRPC worker")
			return
		case <-notify:
			logger.Info(globalCtx, "Reconnecting due to DNS update or failure threshold")
			disconnect(globalCtx)
			connect(globalCtx, cfg.ExternalGRPC)
		default:
			if client != nil {
				ctx, cancel := context.WithTimeout(globalCtx, 3*time.Second)
				defer cancel() // pastikan dilepas

				// --- start parent span
				ctx, span := tracer.Start(ctx, "backend-grpc-request")
				defer span.End()

				// --- inject trace context to metadata ─────────────────────────
				md := metadata.New(nil)
				otel.GetTextMapPropagator().Inject(ctx, telemetry.MetadataTextMapCarrier(md))
				ctx = metadata.NewOutgoingContext(ctx, md)

				// fmt.Println(md.Get("traceparent")[0])
				// fmt.Println(span.SpanContext().TraceID().String())

				// --- call GetAll RPC with metadata ────────────────────────────
				var trailer metadata.MD
				resp, err := client.GetAll(ctx, &emptypb.Empty{}, grpc.Trailer(&trailer))

				// If the server sets the x-trace-id in the trailer, we can log it
				traceID := "empty"
				if ids := trailer.Get("x-trace-id"); len(ids) > 0 {
					traceID = ids[0]
				} else {
					logger.Warn(ctx, "No Trace ID received")
				}

				// --- logging hasil / error ────────────────────────────────────
				if err != nil {
					logger.Error(ctx, "Error calling GetAll",
						slog.String("exception.message", err.Error()),
						slog.String("exception.type", fmt.Sprintf("%T", errors.Unwrap(err))),
						slog.String("exception.stacktrace", string(debug.Stack())),
						slog.String("data.trace_id", traceID),
					)
				} else {
					logger.Info(ctx, "Received products",
						slog.String("data.resolver", resp.Resolver),
						slog.Int("data.count", len(resp.GetProducts())),
						slog.String("data.trace_id", traceID),
					)
				}
			}
			delay := time.Duration(rand.Intn(int(cfg.ClientMaxSleepMs))+1) * time.Millisecond
			time.Sleep(delay)
		}
	}
}

func main() {
	logger.Instance()
	bgCtx := context.Background()
	globalCtx, stop := signal.NotifyContext(bgCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	isProduction := os.Getenv("ENV") == "production"

	logger.Info(globalCtx,
		"Starting gRPC client",
		slog.String("service.name", cfg.AppName),
		slog.String("service.version", version.Version),
		slog.String("service.git_version", version.Commit),
		slog.String("service.build_time", version.BuildTime),
		slog.Bool("service.gracefull_shutdown", isProduction),
	)

	_, _ = telemetry.Instance(globalCtx) // OpenTelemetry setup (no shutdown handling here since it's long-lived client)

	notify := make(chan struct{}, 1)

	// Start DNS watcher goroutine
	go dnsWatcher(globalCtx, cfg.ExternalGRPC, notify)

	// Start gRPC worker loop
	go func() {
		grpcWorker(globalCtx, notify)
	}()

	// Wait for shutdown signal
	<-globalCtx.Done()

	if !isProduction {
		logger.Info(globalCtx, "Received shutdown signal, exiting immediately")
		os.Exit(0)
	} else {
		logger.Info(globalCtx, "Shutting down gRPC client")
		// Let goroutines finish naturally via context cancellation
		time.Sleep(100 * time.Millisecond) // Brief wait for cleanup
	}
}
