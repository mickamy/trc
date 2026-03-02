package capture

import (
	"bufio"
	"net/http"
	"strings"
	"testing"

	"github.com/mickamy/trc/internal/model"
)

// ExtractTraceIDFromHeader exports extractTraceID for testing.
var ExtractTraceIDFromHeader = extractTraceID

// ParseTraceparentValue exports parseTraceparent for testing.
var ParseTraceparentValue = parseTraceparent

// ParseHTTP1 runs the http1Parser on raw input and returns collected records.
func ParseHTTP1(t *testing.T, raw, srcIP, dstIP string) []model.Record {
	t.Helper()

	var records []model.Record
	p := &http1Parser{
		srcIP: srcIP,
		dstIP: dstIP,
		emit: func(r model.Record) {
			records = append(records, r)
		},
	}
	p.run(bufio.NewReader(strings.NewReader(raw)))
	return records
}

// NewHTTP1Parser exports http1Parser construction for testing.
func NewHTTP1Parser(srcIP, dstIP string, emit func(model.Record)) func(r *bufio.Reader) {
	p := &http1Parser{srcIP: srcIP, dstIP: dstIP, emit: emit}
	return p.run
}

// ExtractTraceID exports extractTraceID for external test packages.
func ExtractTraceID(h http.Header) string {
	return extractTraceID(h)
}
