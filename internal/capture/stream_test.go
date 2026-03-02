package capture_test

import (
	"bytes"
	"testing"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	"github.com/mickamy/trc/internal/capture"
	"github.com/mickamy/trc/internal/model"
)

func TestHandleStream_HTTP1Detection(t *testing.T) {
	t.Parallel()

	raw := "GET /health HTTP/1.1\r\n" +
		"Host: svc\r\n" +
		"\r\n" +
		"HTTP/1.1 200 OK\r\n" +
		"Content-Length: 0\r\n" +
		"\r\n"

	records := capture.HandleStream(t, raw, "10.0.0.1", "10.0.0.2")

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Proto != model.ProtoHTTP1 {
		t.Errorf("Proto = %q, want %q", records[0].Proto, model.ProtoHTTP1)
	}
}

func TestHandleStream_HTTP2Detection(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.WriteString("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")

	framer := http2.NewFramer(&buf, nil)

	reqHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":method", Value: "GET"},
		hpack.HeaderField{Name: ":path", Value: "/test"},
		hpack.HeaderField{Name: ":scheme", Value: "http"},
	)
	writeRawHeaders(t, framer, 1, reqHeaders, true)

	respHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":status", Value: "200"},
	)
	writeRawHeaders(t, framer, 1, respHeaders, true)

	records := capture.HandleStream(t, buf.String(), "10.0.0.1", "10.0.0.2")

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Proto != model.ProtoHTTP2 {
		t.Errorf("Proto = %q, want %q", records[0].Proto, model.ProtoHTTP2)
	}
}

func TestHandleStream_GRPCViaH2C(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	buf.WriteString("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")

	framer := http2.NewFramer(&buf, nil)

	reqHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":method", Value: "POST"},
		hpack.HeaderField{Name: ":path", Value: "/auth.Auth/Verify"},
		hpack.HeaderField{Name: ":scheme", Value: "http"},
		hpack.HeaderField{Name: "content-type", Value: "application/grpc"},
	)
	writeRawHeaders(t, framer, 1, reqHeaders, false)

	if err := framer.WriteData(1, true, nil); err != nil {
		t.Fatalf("WriteData() error = %v", err)
	}

	respHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":status", Value: "200"},
		hpack.HeaderField{Name: "content-type", Value: "application/grpc"},
		hpack.HeaderField{Name: "grpc-status", Value: "0"},
	)
	writeRawHeaders(t, framer, 1, respHeaders, true)

	records := capture.HandleStream(t, buf.String(), "10.0.0.1", "10.0.0.2")

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Proto != model.ProtoGRPC {
		t.Errorf("Proto = %q, want %q", records[0].Proto, model.ProtoGRPC)
	}
}

func TestHandleStream_ShortInput(t *testing.T) {
	t.Parallel()

	// Input shorter than h2c preface — should fall back to HTTP/1.1 parser
	// and fail gracefully (no panic, no records).
	records := capture.HandleStream(t, "HI", "10.0.0.1", "10.0.0.2")

	if len(records) != 0 {
		t.Errorf("got %d records, want 0 for short input", len(records))
	}
}
