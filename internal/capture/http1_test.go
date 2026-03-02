package capture_test

import (
	"net/http"
	"testing"

	"github.com/mickamy/trc/internal/capture"
	"github.com/mickamy/trc/internal/model"
)

func TestExtractTraceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers http.Header
		want    string
	}{
		{
			name:    "traceparent",
			headers: http.Header{"Traceparent": []string{"00-abcdef1234567890-1234567890abcdef-01"}},
			want:    "abcdef1234567890",
		},
		{
			name:    "x-trace-id",
			headers: http.Header{"X-Trace-Id": []string{"trace-xyz"}},
			want:    "trace-xyz",
		},
		{
			name:    "x-request-id",
			headers: http.Header{"X-Request-Id": []string{"req-123"}},
			want:    "req-123",
		},
		{
			name: "traceparent takes priority",
			headers: http.Header{
				"Traceparent":  []string{"00-priority-trace-1234567890abcdef-01"},
				"X-Request-Id": []string{"should-not-use"},
			},
			want: "priority",
		},
		{
			name:    "no trace headers",
			headers: http.Header{"Content-Type": []string{"application/json"}},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := capture.ExtractTraceIDFromHeader(tt.headers)
			if got != tt.want {
				t.Errorf("extractTraceID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHTTP1Parser_Run(t *testing.T) {
	t.Parallel()

	raw := "GET /users/42 HTTP/1.1\r\n" +
		"Host: users\r\n" +
		"X-Request-Id: req-abc\r\n" +
		"\r\n" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Length: 13\r\n" +
		"\r\n" +
		`{"id":"42"}` + "\r\n"

	records := capture.ParseHTTP1(t, raw, "10.0.0.1", "10.0.0.2")

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	if r.Proto != model.ProtoHTTP1 {
		t.Errorf("Proto = %q, want %q", r.Proto, model.ProtoHTTP1)
	}
	if r.Method != http.MethodGet {
		t.Errorf("Method = %q, want %q", r.Method, http.MethodGet)
	}
	if r.Path != "/users/42" {
		t.Errorf("Path = %q, want %q", r.Path, "/users/42")
	}
	if r.Status != 200 {
		t.Errorf("Status = %d, want %d", r.Status, 200)
	}
	if r.TraceID != "req-abc" {
		t.Errorf("TraceID = %q, want %q", r.TraceID, "req-abc")
	}
	if r.SrcIP != "10.0.0.1" {
		t.Errorf("SrcIP = %q, want %q", r.SrcIP, "10.0.0.1")
	}
	if r.DstIP != "10.0.0.2" {
		t.Errorf("DstIP = %q, want %q", r.DstIP, "10.0.0.2")
	}
}

func TestHTTP1Parser_MultipleExchanges(t *testing.T) {
	t.Parallel()

	raw := "GET /a HTTP/1.1\r\nHost: svc\r\n\r\n" +
		"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n" +
		"POST /b HTTP/1.1\r\nHost: svc\r\nContent-Length: 0\r\n\r\n" +
		"HTTP/1.1 201 Created\r\nContent-Length: 0\r\n\r\n"

	records := capture.ParseHTTP1(t, raw, "10.0.0.1", "10.0.0.2")

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	if records[0].Method != http.MethodGet || records[0].Path != "/a" {
		t.Errorf("record[0] = %s %s, want GET /a", records[0].Method, records[0].Path)
	}
	if records[1].Method != http.MethodPost || records[1].Path != "/b" {
		t.Errorf("record[1] = %s %s, want POST /b", records[1].Method, records[1].Path)
	}
}

func TestParseTraceparent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard format",
			input: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			want:  "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			name:  "no dashes",
			input: "invalid",
			want:  "invalid",
		},
		{
			name:  "single dash",
			input: "00-traceid",
			want:  "traceid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := capture.ParseTraceparentValue(tt.input)
			if got != tt.want {
				t.Errorf("parseTraceparent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
