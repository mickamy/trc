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
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		return
	}
	req.Header.Set("X-Request-Id", fmt.Sprintf("trc-%d", time.Now().UnixNano()))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	fmt.Fprintf(os.Stderr, "client: %s %d\n", url, resp.StatusCode)
}
