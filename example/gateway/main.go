package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
)

// rawCodec passes raw bytes without protobuf serialization.
type rawCodec struct{}

func (rawCodec) Marshal(v any) ([]byte, error)     { return v.([]byte), nil }
func (rawCodec) Unmarshal(data []byte, v any) error { *v.(*[]byte) = data; return nil }
func (rawCodec) Name() string                       { return "raw" }

func init() { encoding.RegisterCodec(rawCodec{}) }

func main() {
	http.HandleFunc("/api/orders", handleAPIOrders)
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	fmt.Fprintln(os.Stderr, "gateway: listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Fprintf(os.Stderr, "gateway: %v\n", err)
		os.Exit(1)
	}
}

func handleAPIOrders(w http.ResponseWriter, r *http.Request) {
	traceID := r.Header.Get("X-Request-Id")
	if traceID == "" {
		traceID = fmt.Sprintf("trc-%d", time.Now().UnixNano())
	}

	// 1. Call auth service (gRPC).
	authResult, err := callAuth(r.Context(), traceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("auth failed: %v", err), http.StatusUnauthorized)
		return
	}
	_ = authResult

	// 2. Call orders service (h2c / HTTP/2).
	orderResult, err := callOrders(r.Context(), traceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("orders failed: %v", err), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"trace_id": traceID,
		"order":    orderResult,
	})
}

func callAuth(ctx context.Context, traceID string) (string, error) {
	authAddr := os.Getenv("AUTH_ADDR")
	if authAddr == "" {
		authAddr = "auth:50051"
	}

	conn, err := grpc.NewClient(
		"dns:///"+authAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return "", fmt.Errorf("connecting to auth: %w", err)
	}
	defer conn.Close()

	// Pass trace-id as gRPC metadata.
	ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", traceID)

	// Call Verify using the raw client invoke (no proto needed).
	var result []byte
	err = conn.Invoke(ctx, "/auth.Auth/Verify", []byte(`{"token":"test"}`), &result, grpc.ForceCodec(rawCodec{}))
	if err != nil {
		return "", fmt.Errorf("calling auth.Verify: %w", err)
	}

	return string(result), nil
}

func callOrders(ctx context.Context, traceID string) (string, error) {
	ordersURL := os.Getenv("ORDERS_URL")
	if ordersURL == "" {
		ordersURL = "http://orders:8080"
	}

	// Create h2c client for cleartext HTTP/2.
	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ordersURL+"/orders", nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Request-Id", traceID)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling orders: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading orders response: %w", err)
	}

	return string(body), nil
}
