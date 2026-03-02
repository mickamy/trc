package capture_test

import (
	"bytes"
	"testing"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	"github.com/mickamy/trc/internal/capture"
	"github.com/mickamy/trc/internal/model"
)

func TestHandleStreamPair_HTTP1Detection(t *testing.T) {
	t.Parallel()

	clientRaw := "GET /health HTTP/1.1\r\n" +
		"Host: svc\r\n" +
		"\r\n"

	serverRaw := "HTTP/1.1 200 OK\r\n" +
		"Content-Length: 0\r\n" +
		"\r\n"

	records := capture.HandleStreamPair(
		t, clientRaw, serverRaw, "10.0.0.1", "10.0.0.2",
	)

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Proto != model.ProtoHTTP1 {
		t.Errorf("Proto = %q, want %q", records[0].Proto, model.ProtoHTTP1)
	}
}

func TestHandleStreamPair_HTTP2Detection(t *testing.T) {
	t.Parallel()

	// Client direction: h2c preface + request HEADERS.
	var clientBuf bytes.Buffer
	clientBuf.WriteString("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
	clientFramer := http2.NewFramer(&clientBuf, nil)
	reqHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":method", Value: "GET"},
		hpack.HeaderField{Name: ":path", Value: "/test"},
		hpack.HeaderField{Name: ":scheme", Value: "http"},
	)
	writeRawHeaders(t, clientFramer, 1, reqHeaders, true)

	// Server direction: response HEADERS.
	var serverBuf bytes.Buffer
	serverFramer := http2.NewFramer(&serverBuf, nil)
	respHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":status", Value: "200"},
	)
	writeRawHeaders(t, serverFramer, 1, respHeaders, true)

	records := capture.HandleStreamPair(
		t, clientBuf.String(), serverBuf.String(), "10.0.0.1", "10.0.0.2",
	)

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Proto != model.ProtoHTTP2 {
		t.Errorf("Proto = %q, want %q", records[0].Proto, model.ProtoHTTP2)
	}
}

func TestHandleStreamPair_GRPCViaH2C(t *testing.T) {
	t.Parallel()

	// Client direction: h2c preface + gRPC request + DATA.
	var clientBuf bytes.Buffer
	clientBuf.WriteString("PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n")
	clientFramer := http2.NewFramer(&clientBuf, nil)
	reqHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":method", Value: "POST"},
		hpack.HeaderField{Name: ":path", Value: "/auth.Auth/Verify"},
		hpack.HeaderField{Name: ":scheme", Value: "http"},
		hpack.HeaderField{Name: "content-type", Value: "application/grpc"},
	)
	writeRawHeaders(t, clientFramer, 1, reqHeaders, false)
	if err := clientFramer.WriteData(1, true, nil); err != nil {
		t.Fatalf("WriteData() error = %v", err)
	}

	// Server direction: response HEADERS with grpc-status trailer.
	var serverBuf bytes.Buffer
	serverFramer := http2.NewFramer(&serverBuf, nil)
	respHeaders := encodeHeaders(t,
		hpack.HeaderField{Name: ":status", Value: "200"},
		hpack.HeaderField{Name: "content-type", Value: "application/grpc"},
		hpack.HeaderField{Name: "grpc-status", Value: "0"},
	)
	writeRawHeaders(t, serverFramer, 1, respHeaders, true)

	records := capture.HandleStreamPair(
		t, clientBuf.String(), serverBuf.String(), "10.0.0.1", "10.0.0.2",
	)

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Proto != model.ProtoGRPC {
		t.Errorf("Proto = %q, want %q", records[0].Proto, model.ProtoGRPC)
	}
}

func TestHandleStreamPair_ShortInput(t *testing.T) {
	t.Parallel()

	// Input that can't be identified as any protocol — should return
	// immediately with no records and no panic.
	records := capture.HandleStreamPair(
		t, "HI", "", "10.0.0.1", "10.0.0.2",
	)

	if len(records) != 0 {
		t.Errorf("got %d records, want 0 for short input", len(records))
	}
}
