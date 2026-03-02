package capture

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickamy/trc/internal/model"
)

// ExtractTraceIDFromHeader exports extractTraceID for testing.
var ExtractTraceIDFromHeader = extractTraceID

// ParseTraceparentValue exports parseTraceparent for testing.
var ParseTraceparentValue = parseTraceparent

// ParseHTTP1 runs the HTTP/1.1 request and response parsers on separate
// input streams and returns collected records.
func ParseHTTP1(
	t *testing.T,
	reqRaw, respRaw, clientIP, serverIP string,
) []model.Record {
	t.Helper()

	cs := newConnState(clientIP, serverIP)
	var mu sync.Mutex
	var records []model.Record
	emit := func(r model.Record) {
		mu.Lock()
		records = append(records, r)
		mu.Unlock()
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		runHTTP1Request(bufio.NewReader(strings.NewReader(reqRaw)), cs, time.Now)
	}()
	go func() {
		defer wg.Done()
		runHTTP1Response(
			bufio.NewReader(strings.NewReader(respRaw)),
			cs, clientIP, serverIP, emit, time.Now,
		)
	}()
	wg.Wait()
	return records
}

// ExtractTraceID exports extractTraceID for external test packages.
func ExtractTraceID(h http.Header) string {
	return extractTraceID(h)
}

// ParseHTTP2 runs the http2Parser on split client/server input and returns
// collected records.
func ParseHTTP2(t *testing.T, client, server io.Reader, srcIP, dstIP string) []model.Record {
	t.Helper()

	var mu sync.Mutex
	var records []model.Record
	p := newHTTP2Parser(srcIP, dstIP, func(rec model.Record) {
		mu.Lock()
		records = append(records, rec)
		mu.Unlock()
	})
	p.runBothDirections(client, server, time.Now)
	return records
}

// GRPCMethod exports grpcMethod for testing.
var GRPCMethod = grpcMethod

// GRPCStatusName exports grpcStatusName for testing.
var GRPCStatusName = grpcStatusName

// IsGRPC exports isGRPC for testing.
var IsGRPC = isGRPC

// HandleStreamPair runs both directions of a connection through protocol
// detection and returns collected records.
func HandleStreamPair(
	t *testing.T,
	clientRaw, serverRaw, clientIP, serverIP string,
) []model.Record {
	t.Helper()

	var mu sync.Mutex
	var records []model.Record
	emit := func(r model.Record) {
		mu.Lock()
		records = append(records, r)
		mu.Unlock()
	}

	clientBR := bufio.NewReaderSize(
		strings.NewReader(clientRaw), 4096, //nolint:mnd
	)
	serverBR := bufio.NewReaderSize(
		strings.NewReader(serverRaw), 4096, //nolint:mnd
	)

	// Detect protocol from client direction.
	peek, _ := clientBR.Peek(h2cPrefaceLen)
	s := string(peek)

	var wg sync.WaitGroup

	switch {
	case len(peek) >= h2cPrefaceLen && s == h2cPreface:
		// HTTP/2
		_, _ = clientBR.Discard(h2cPrefaceLen)
		p := newHTTP2Parser(clientIP, serverIP, emit)
		wg.Add(2)
		go func() {
			defer wg.Done()
			p.runDirection(clientBR, dirClient, time.Now)
		}()
		go func() {
			defer wg.Done()
			p.runDirection(serverBR, dirServer, time.Now)
		}()
	case looksLikeHTTPMethod(s):
		// HTTP/1.1
		cs := newConnState(clientIP, serverIP)
		wg.Add(2)
		go func() {
			defer wg.Done()
			runHTTP1Request(clientBR, cs, time.Now)
		}()
		go func() {
			defer wg.Done()
			runHTTP1Response(serverBR, cs, clientIP, serverIP, emit, time.Now)
		}()
	default:
		// Unknown protocol — return empty.
	}

	wg.Wait()
	return records
}
