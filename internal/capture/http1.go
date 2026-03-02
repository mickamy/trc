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

// http1Parser reads HTTP/1.1 request/response pairs from a buffered reader
// and emits Records for each completed exchange.
type http1Parser struct {
	srcIP string
	dstIP string
	emit  func(model.Record)
}

// run reads HTTP/1.1 request/response pairs from r until EOF or error.
func (p *http1Parser) run(r *bufio.Reader) {
	for {
		reqStart := time.Now()

		req, err := http.ReadRequest(r)
		if err != nil {
			return
		}

		// Drain and close the request body so the reader advances
		// past it to the next message boundary.
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()

		traceID := extractTraceID(req.Header)

		resp, err := http.ReadResponse(bufio.NewReader(r), req)
		if err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		duration := time.Since(reqStart)

		p.emit(model.Record{
			Timestamp:  reqStart,
			Proto:      model.ProtoHTTP1,
			SrcIP:      p.srcIP,
			DstIP:      p.dstIP,
			Method:     req.Method,
			Path:       req.URL.RequestURI(),
			Status:     resp.StatusCode,
			DurationMs: float64(duration.Milliseconds()),
			TraceID:    traceID,
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
