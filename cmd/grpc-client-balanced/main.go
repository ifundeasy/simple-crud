package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"simple-crud/internal/config"
	pb "simple-crud/internal/handler/grpc/pb"
	"simple-crud/internal/logger"

	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	conn   *grpc.ClientConn
	client pb.ProductServiceClient
	log    = logger.Instance()
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
func connect(target string) error {
	var err error
	conn, err = grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		log.Error("Failed to dial gRPC", slog.String("error", err.Error()))
		return err
	}
	client = pb.NewProductServiceClient(conn)
	log.Info("gRPC connected", slog.String("target", target))
	return nil
}

// disconnect closes the gRPC connection.
func disconnect() {
	if conn != nil {
		conn.Close()
		log.Info("gRPC connection closed")
	}
}

// dnsWatcher periodically resolves DNS and notifies the gRPC worker if any changes detected.
func dnsWatcher(host string, notify chan struct{}) {
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
		log.Info("Just keep watching gRPC DNS..", slog.Any("host", cleanHost))
		addrs, err := resolvePods(cleanHost)
		if err != nil {
			// DNS lookup failed
			log.Error("DNS lookup failed", slog.Any("host", cleanHost), slog.String("error", err.Error()))
			consecutiveFail++

			// Go's built-in net.LookupHost() does not implement any DNS retry, failover, or backoff strategy.
			if consecutiveFail >= 3 {
				log.Warn("DNS lookup failed 3 times consecutively, forcing reconnect", slog.Any("host", cleanHost))
				notify <- struct{}{}
				consecutiveFail = 0
			}
			time.Sleep(time.Duration(cfg.DnsResolverDelayMs) * time.Millisecond)
			continue
		}

		// DNS lookup succeeded → reset failure counter
		consecutiveFail = 0

		// Calculate hash of resolved pod IP addresses
		sort.Strings(addrs) // Sort it first! this is important to prevent same items but with random index
		newHash := hashAddresses(addrs)
		if newHash != lastHash {
			log.Info("Detected backend pod change", slog.Any("host", cleanHost), slog.Any("addresses", addrs))
			lastHash = newHash
			notify <- struct{}{}
		}

		// Wait before next DNS check
		time.Sleep(time.Duration(cfg.DnsResolverDelayMs) * time.Millisecond)
	}
}

// grpcWorker performs gRPC request loop and reconnects whenever DNS watcher sends signal.
func grpcWorker(notify chan struct{}) {
	defer disconnect()
	err := connect(cfg.ExternalGRPC)
	if err != nil {
		log.Error("Initial gRPC connection failed, will retry on next DNS change")
	}

	for {
		select {
		case <-notify:
			log.Info("Reconnecting due to DNS update or failure threshold")
			disconnect()
			connect(cfg.ExternalGRPC)

		default:
			if client != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				resp, err := client.GetAll(ctx, &emptypb.Empty{})
				cancel()

				if err != nil {
					log.Error("Error calling GetAll", slog.String("error", err.Error()))
				} else {
					log.Info("Received products",
						slog.String("resolver", resp.Resolver),
						slog.Int("count", len(resp.GetProducts())),
					)
				}
			}
			time.Sleep(time.Duration(cfg.AppClientDelayMs) * time.Millisecond)
		}
	}
}

func main() {
	notify := make(chan struct{}, 1)

	// Start DNS watcher goroutine
	go dnsWatcher(cfg.ExternalGRPC, notify)

	// Start gRPC worker loop
	grpcWorker(notify)
}
