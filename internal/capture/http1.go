package capture

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mickamy/trc/internal/model"
)

// traceHeaders lists the headers to check for trace ID extraction, in priority order.
var traceHeaders = []string{
	"Traceparent",
	"X-Trace-Id",
	"X-Request-Id",
}

// runHTTP1Request reads HTTP/1.1 requests from the client→server stream
// and sends them to cs.reqCh for pairing with responses.
// It closes cs.reqCh on return to unblock the response reader.
func runHTTP1Request(r *bufio.Reader, cs *connState) {
	defer close(cs.reqCh)

	for {
		req, err := http.ReadRequest(r)
		if err != nil {
			return
		}

		traceID := extractTraceID(req.Header)

		// Drain and close the request body so the reader advances
		// past it to the next message boundary.
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()

		select {
		case cs.reqCh <- h1Pending{
			method:  req.Method,
			path:    req.URL.RequestURI(),
			traceID: traceID,
			start:   time.Now(),
		}:
		default:
			// Channel full — drop to avoid blocking.
		}
	}
}

// runHTTP1Response reads HTTP/1.1 responses from the server→client stream,
// pairs them with requests from cs.reqCh, and emits records.
func runHTTP1Response(
	r *bufio.Reader, cs *connState,
	clientIP, serverIP string, emit func(model.Record),
) {
	for {
		resp, err := http.ReadResponse(r, nil)
		if err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		pending, ok := <-cs.reqCh
		if !ok {
			return
		}

		duration := time.Since(pending.start)
		emit(model.Record{
			Timestamp:  pending.start,
			Proto:      model.ProtoHTTP1,
			SrcIP:      clientIP,
			DstIP:      serverIP,
			Method:     pending.method,
			Path:       pending.path,
			Status:     resp.StatusCode,
			DurationMs: float64(duration.Milliseconds()),
			TraceID:    pending.traceID,
		})
	}
}

// extractTraceID returns the first non-empty trace header value found.
func extractTraceID(h http.Header) string {
	for _, key := range traceHeaders {
		if v := h.Get(key); v != "" {
			// Traceparent format: "00-<trace-id>-<parent-id>-<flags>"
			if strings.EqualFold(key, "Traceparent") {
				return parseTraceparent(v)
			}
			return v
		}
	}
	return ""
}

// parseTraceparent extracts the trace-id portion from a W3C Traceparent header.
// Format: version-traceid-parentid-traceflags (e.g. "00-abc123-def456-01").
func parseTraceparent(v string) string {
	parts := strings.SplitN(v, "-", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[1]
}
