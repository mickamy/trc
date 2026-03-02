package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", handleCreateOrder)

	h2s := &http2.Server{}
	handler := h2c.NewHandler(mux, h2s)

	fmt.Fprintln(os.Stderr, "orders: listening on :8080 (h2c)")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		fmt.Fprintf(os.Stderr, "orders: %v\n", err)
		os.Exit(1)
	}
}

func handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	// Call users service via HTTP/1.1.
	usersURL := os.Getenv("USERS_URL")
	if usersURL == "" {
		usersURL = "http://users:8080"
	}

	resp, err := http.Get(usersURL + "/users/42")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to call users: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"order_id": "order-001",
		"user_id":  "42",
		"status":   "created",
	})
}
