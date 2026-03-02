package capture_test

import (
	"bytes"
	"net/http"
	"testing"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	"github.com/mickamy/trc/internal/capture"
	"github.com/mickamy/trc/internal/model"
)

func TestGRPCMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "full path with package",
			path: "/auth.Auth/Verify",
			want: "Auth/Verify",
		},
		{
			name: "no package prefix",
			path: "/Auth/Verify",
			want: "Auth/Verify",
		},
		{
			name: "deeply nested package",
			path: "/com.example.auth.Auth/Verify",
			want: "Auth/Verify",
		},
		{
			name: "no slash",
			path: "/Method",
			want: "Method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := capture.GRPCMethod(tt.path)
			if got != tt.want {
				t.Errorf("grpcMethod(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestGRPCStatusName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want string
	}{
		{"0", "OK"},
		{"1", "CANCELLED"},
		{"5", "NOT_FOUND"},
		{"13", "INTERNAL"},
		{"16", "UNAUTHENTICATED"},
		{"99", "CODE_99"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()

			got := capture.GRPCStatusName(tt.code)
			if got != tt.want {
				t.Errorf("grpcStatusName(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestIsGRPC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		contentType string
		want        bool
	}{
		{"application/grpc", true},
		{"application/grpc+proto", true},
		{"application/json", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			t.Parallel()

			got := capture.IsGRPC(tt.contentType)
			if got != tt.want {
				t.Errorf("isGRPC(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}

func TestHTTP2Parser_SimpleExchange(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	framer := http2.NewFramer(&buf, nil)

	// Encode request HEADERS: :method=GET, :path=/users/42
	reqHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":method", Value: "GET"},
		hpack.HeaderField{Name: ":path", Value: "/users/42"},
		hpack.HeaderField{Name: ":scheme", Value: "http"},
		hpack.HeaderField{Name: "x-request-id", Value: "trace-h2"},
	)
	writeRawHeaders(t, framer, 1, reqHeaders, true)

	// Encode response HEADERS: :status=200
	respHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":status", Value: "200"},
	)
	writeRawHeaders(t, framer, 1, respHeaders, true)

	records := capture.ParseHTTP2(t, &buf, "10.0.0.1", "10.0.0.2")

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	if r.Proto != model.ProtoHTTP2 {
		t.Errorf("Proto = %q, want %q", r.Proto, model.ProtoHTTP2)
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
	if r.TraceID != "trace-h2" {
		t.Errorf("TraceID = %q, want %q", r.TraceID, "trace-h2")
	}
}

func TestHTTP2Parser_GRPCDetection(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	framer := http2.NewFramer(&buf, nil)

	// gRPC request
	reqHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":method", Value: "POST"},
		hpack.HeaderField{Name: ":path", Value: "/auth.Auth/Verify"},
		hpack.HeaderField{Name: ":scheme", Value: "http"},
		hpack.HeaderField{Name: "content-type", Value: "application/grpc"},
	)
	writeRawHeaders(t, framer, 3, reqHeaders, false)

	// Empty DATA with END_STREAM
	if err := framer.WriteData(3, true, nil); err != nil {
		t.Fatalf("WriteData() error = %v", err)
	}

	// gRPC response headers + trailers
	respHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":status", Value: "200"},
		hpack.HeaderField{Name: "content-type", Value: "application/grpc"},
		hpack.HeaderField{Name: "grpc-status", Value: "0"},
	)
	writeRawHeaders(t, framer, 3, respHeaders, true)

	records := capture.ParseHTTP2(t, &buf, "10.0.0.1", "10.0.0.2")

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	if r.Proto != model.ProtoGRPC {
		t.Errorf("Proto = %q, want %q", r.Proto, model.ProtoGRPC)
	}
	if r.Method != "Auth/Verify" {
		t.Errorf("Method = %q, want %q", r.Method, "Auth/Verify")
	}
	if r.GRPCStatus != "OK" {
		t.Errorf("GRPCStatus = %q, want %q", r.GRPCStatus, "OK")
	}
}

// encodeHeaders encodes header fields using HPACK.
func encodeHeaders(t *testing.T, fields ...hpack.HeaderField) []byte {
	t.Helper()

	var buf bytes.Buffer
	enc := hpack.NewEncoder(&buf)
	for _, f := range fields {
		if err := enc.WriteField(f); err != nil {
			t.Fatalf("WriteField(%q, %q) error = %v", f.Name, f.Value, err)
		}
	}
	return buf.Bytes()
}

// writeRawHeaders writes a HEADERS frame with END_HEADERS always set.
func writeRawHeaders(
	t *testing.T, framer *http2.Framer, streamID uint32,
	headerBlock []byte, endStream bool,
) {
	t.Helper()

	flags := http2.FlagHeadersEndHeaders
	if endStream {
		flags |= http2.FlagHeadersEndStream
	}

	if err := framer.WriteRawFrame(http2.FrameHeaders, flags, streamID, headerBlock); err != nil {
		t.Fatalf("WriteRawFrame() error = %v", err)
	}
}
