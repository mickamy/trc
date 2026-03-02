package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://gateway:8080"
	}

	interval := 3 * time.Second

	fmt.Fprintf(os.Stderr, "client: polling %s every %s\n", gatewayURL, interval)

	for {
		call(gatewayURL + "/api/orders")
		time.Sleep(interval)
	}
}

func call(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	fmt.Fprintf(os.Stderr, "client: %s %d\n", url, resp.StatusCode)
}
