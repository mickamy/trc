package capture

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	"github.com/mickamy/trc/internal/model"
)

// direction indicates which side of a TCP connection a stream belongs to.
type direction int

const (
	dirClient direction = iota
	dirServer
)

// streamState tracks a single HTTP/2 stream's request/response progress.
type streamState struct {
	startTime   time.Time
	method      string
	path        string
	status      int
	contentType string
	grpcStatus  string
	traceID     string
	// headerBuf accumulates HEADERS/CONTINUATION fragments until END_HEADERS.
	headerBuf []byte
	// endStreamSeen is true once an END_STREAM flag has been observed.
	endStreamSeen bool
}

// http2Parser reads HTTP/2 frames from both directions of a connection
// and emits Records. It is safe for concurrent use from two goroutines
// (one per direction).
type http2Parser struct {
	srcIP   string
	dstIP   string
	emit    func(model.Record)
	mu      sync.Mutex
	streams map[uint32]*streamState
}

func newHTTP2Parser(srcIP, dstIP string, emit func(model.Record)) *http2Parser {
	return &http2Parser{
		srcIP:   srcIP,
		dstIP:   dstIP,
		emit:    emit,
		streams: make(map[uint32]*streamState),
	}
}

// run reads HTTP/2 frames from r (single direction). Backward compatible
// entry point used by tests.
func (p *http2Parser) run(r io.Reader) {
	p.runDirection(r, dirClient)
}

// runDirection reads HTTP/2 frames from r for the given direction until
// EOF or error. Each direction maintains its own HPACK decoder.
func (p *http2Parser) runDirection(r io.Reader, _ direction) {
	framer := http2.NewFramer(io.Discard, r)
	framer.ReadMetaHeaders = nil // we decode HPACK ourselves
	// Allow large frames in captures without erroring.
	framer.SetMaxReadFrameSize(1 << 24) //nolint:mnd // 16 MiB max frame

	var activeStreamID uint32
	decoder := hpack.NewDecoder(4096, func(f hpack.HeaderField) { //nolint:mnd // HPACK default table size
		// Called during decoder.Write inside processFrame, mutex already held.
		s, ok := p.streams[activeStreamID]
		if !ok {
			return
		}
		switch f.Name {
		case ":method":
			s.method = f.Value
		case ":path":
			s.path = f.Value
		case ":status":
			s.status, _ = strconv.Atoi(f.Value)
		case "content-type":
			s.contentType = f.Value
		case "grpc-status":
			s.grpcStatus = f.Value
		case "traceparent":
			s.traceID = parseTraceparent(f.Value)
		case "x-trace-id":
			if s.traceID == "" {
				s.traceID = f.Value
			}
		case "x-request-id":
			if s.traceID == "" {
				s.traceID = f.Value
			}
		}
	})

	for {
		f, err := framer.ReadFrame()
		if err != nil {
			return
		}
		p.processFrame(f, decoder, &activeStreamID)
	}
}

func (p *http2Parser) processFrame(
	f http2.Frame, decoder *hpack.Decoder, activeStreamID *uint32,
) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch frame := f.(type) {
	case *http2.HeadersFrame:
		p.handleHeaders(frame, decoder, activeStreamID)
	case *http2.ContinuationFrame:
		p.handleContinuation(frame, decoder, activeStreamID)
	case *http2.DataFrame:
		p.handleData(frame)
	default:
		// SETTINGS, WINDOW_UPDATE, PING, etc. — skip
	}
}

func (p *http2Parser) handleHeaders(
	f *http2.HeadersFrame, decoder *hpack.Decoder, activeStreamID *uint32,
) {
	id := f.StreamID
	s, ok := p.streams[id]
	if !ok {
		s = &streamState{startTime: time.Now()}
		p.streams[id] = s
	}

	s.headerBuf = append(s.headerBuf, f.HeaderBlockFragment()...)

	if f.HeadersEnded() {
		p.decodeHeaders(id, decoder, activeStreamID)
	}

	if f.StreamEnded() {
		p.finishStream(id)
	}
}

func (p *http2Parser) handleContinuation(
	f *http2.ContinuationFrame, decoder *hpack.Decoder, activeStreamID *uint32,
) {
	id := f.StreamID
	s, ok := p.streams[id]
	if !ok {
		return
	}

	s.headerBuf = append(s.headerBuf, f.HeaderBlockFragment()...)

	if f.HeadersEnded() {
		p.decodeHeaders(id, decoder, activeStreamID)
	}
}

func (p *http2Parser) handleData(f *http2.DataFrame) {
	if f.StreamEnded() {
		p.finishStream(f.StreamID)
	}
}

func (p *http2Parser) decodeHeaders(
	id uint32, decoder *hpack.Decoder, activeStreamID *uint32,
) {
	s, ok := p.streams[id]
	if !ok {
		return
	}

	*activeStreamID = id
	if _, err := decoder.Write(s.headerBuf); err != nil {
		// Malformed headers — skip this stream.
		delete(p.streams, id)
		return
	}
	s.headerBuf = nil

	// If END_STREAM was already received and both sides are now complete, emit.
	if s.endStreamSeen {
		p.finishStream(id)
	}
}

func (p *http2Parser) finishStream(id uint32) {
	s, ok := p.streams[id]
	if !ok {
		return
	}

	s.endStreamSeen = true

	// We need both method (request) and status (response) to emit a record.
	// If incomplete, keep the entry until the other side arrives.
	if s.method == "" || s.status == 0 {
		return
	}
	delete(p.streams, id)

	proto := model.ProtoHTTP2
	method := s.method
	grpcStatus := ""

	if isGRPC(s.contentType) {
		proto = model.ProtoGRPC
		method = grpcMethod(s.path)
		grpcStatus = grpcStatusName(s.grpcStatus)
	}

	duration := time.Since(s.startTime)

	p.emit(model.Record{
		Timestamp:  s.startTime,
		Proto:      proto,
		SrcIP:      p.srcIP,
		DstIP:      p.dstIP,
		Method:     method,
		Path:       s.path,
		Status:     s.status,
		GRPCStatus: grpcStatus,
		DurationMs: float64(duration.Milliseconds()),
		TraceID:    s.traceID,
	})
}

// isGRPC reports whether the content-type indicates a gRPC request.
func isGRPC(contentType string) bool {
	return strings.HasPrefix(contentType, "application/grpc")
}

// grpcMethod extracts "Service/Method" from a gRPC path like "/package.Service/Method".
func grpcMethod(path string) string {
	// Remove leading slash.
	trimmed := strings.TrimPrefix(path, "/")

	// Find the service part (last component before the method).
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return trimmed
	}

	service := parts[0]
	method := parts[1]

	// Strip the package prefix from the service name: "pkg.Service" -> "Service".
	if idx := strings.LastIndex(service, "."); idx >= 0 {
		service = service[idx+1:]
	}

	return fmt.Sprintf("%s/%s", service, method)
}

// grpcStatusName maps a gRPC status code string to its name.
func grpcStatusName(code string) string {
	names := map[string]string{
		"0":  "OK",
		"1":  "CANCELLED",
		"2":  "UNKNOWN",
		"3":  "INVALID_ARGUMENT",
		"4":  "DEADLINE_EXCEEDED",
		"5":  "NOT_FOUND",
		"6":  "ALREADY_EXISTS",
		"7":  "PERMISSION_DENIED",
		"8":  "RESOURCE_EXHAUSTED",
		"9":  "FAILED_PRECONDITION",
		"10": "ABORTED",
		"11": "OUT_OF_RANGE",
		"12": "UNIMPLEMENTED",
		"13": "INTERNAL",
		"14": "UNAVAILABLE",
		"15": "DATA_LOSS",
		"16": "UNAUTHENTICATED",
	}
	if n, ok := names[code]; ok {
		return n
	}
	if code == "" {
		return ""
	}
	return "CODE_" + code
}
