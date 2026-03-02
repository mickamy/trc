package capture

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	"github.com/mickamy/trc/internal/model"
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
}

// http2Parser reads HTTP/2 frames from a reader and emits Records.
type http2Parser struct {
	srcIP   string
	dstIP   string
	emit    func(model.Record)
	streams map[uint32]*streamState
	decoder *hpack.Decoder
	// activeStreamID is set before each HPACK decode so the onHeader callback
	// knows which stream to update. Only accessed from the run goroutine.
	activeStreamID uint32
}

func newHTTP2Parser(srcIP, dstIP string, emit func(model.Record)) *http2Parser {
	p := &http2Parser{
		srcIP:   srcIP,
		dstIP:   dstIP,
		emit:    emit,
		streams: make(map[uint32]*streamState),
	}
	p.decoder = hpack.NewDecoder(4096, p.onHeader)
	return p
}

// run reads HTTP/2 frames from r until EOF or error.
func (p *http2Parser) run(r io.Reader) {
	framer := http2.NewFramer(io.Discard, r)
	framer.ReadMetaHeaders = nil // we decode HPACK ourselves
	// Allow large frames in captures without erroring.
	framer.SetMaxReadFrameSize(1 << 24) //nolint:mnd // 16 MiB max frame

	for {
		f, err := framer.ReadFrame()
		if err != nil {
			return
		}

		switch frame := f.(type) {
		case *http2.HeadersFrame:
			p.handleHeaders(frame)
		case *http2.ContinuationFrame:
			p.handleContinuation(frame)
		case *http2.DataFrame:
			p.handleData(frame)
		default:
			// SETTINGS, WINDOW_UPDATE, PING, etc. — skip
		}
	}
}

func (p *http2Parser) handleHeaders(f *http2.HeadersFrame) {
	id := f.StreamID
	s, ok := p.streams[id]
	if !ok {
		s = &streamState{startTime: time.Now()}
		p.streams[id] = s
	}

	s.headerBuf = append(s.headerBuf, f.HeaderBlockFragment()...)

	if f.HeadersEnded() {
		p.decodeHeaders(id)
	}

	if f.StreamEnded() {
		p.finishStream(id)
	}
}

func (p *http2Parser) handleContinuation(f *http2.ContinuationFrame) {
	id := f.StreamID
	s, ok := p.streams[id]
	if !ok {
		return
	}

	s.headerBuf = append(s.headerBuf, f.HeaderBlockFragment()...)

	if f.HeadersEnded() {
		p.decodeHeaders(id)
	}
}

func (p *http2Parser) handleData(f *http2.DataFrame) {
	if f.StreamEnded() {
		p.finishStream(f.StreamID)
	}
}

func (p *http2Parser) decodeHeaders(id uint32) {
	s, ok := p.streams[id]
	if !ok {
		return
	}

	p.activeStreamID = id
	if _, err := p.decoder.Write(s.headerBuf); err != nil {
		// Malformed headers — skip this stream.
		delete(p.streams, id)
		return
	}
	s.headerBuf = nil
}

// onHeader is the HPACK callback invoked for each decoded header field.
func (p *http2Parser) onHeader(f hpack.HeaderField) {
	s, ok := p.streams[p.activeStreamID]
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
}

func (p *http2Parser) finishStream(id uint32) {
	s, ok := p.streams[id]
	if !ok {
		return
	}

	// We need both method (request) and status (response) to emit a record.
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
