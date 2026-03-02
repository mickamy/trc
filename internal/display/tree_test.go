package display_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickamy/trc/internal/display"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

func TestPrintTree_BasicChain(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	records := []model.Record{
		{
			Timestamp: base, Proto: model.ProtoHTTP1,
			SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
			Method: "GET", Path: "/api/orders",
			Status: 500, DurationMs: 52, TraceID: "abc-123",
		},
		{
			Timestamp: base.Add(1 * time.Millisecond), Proto: model.ProtoGRPC,
			SrcIP: "10.0.0.2", DstIP: "10.0.0.3",
			Method: "Auth/Verify", Path: "/auth.Auth/Verify",
			Status: 200, GRPCStatus: "OK", DurationMs: 3, TraceID: "abc-123",
		},
		{
			Timestamp: base.Add(5 * time.Millisecond), Proto: model.ProtoHTTP2,
			SrcIP: "10.0.0.2", DstIP: "10.0.0.4",
			Method: "POST", Path: "/orders",
			Status: 500, DurationMs: 45, TraceID: "abc-123",
		},
		{
			Timestamp: base.Add(10 * time.Millisecond), Proto: model.ProtoHTTP1,
			SrcIP: "10.0.0.4", DstIP: "10.0.0.5",
			Method: "GET", Path: "/users/42",
			Status: 200, DurationMs: 12, TraceID: "abc-123",
		},
	}

	svcMap := docker.ServiceMap{
		"10.0.0.1": "client",
		"10.0.0.2": "gateway",
		"10.0.0.3": "auth",
		"10.0.0.4": "orders",
		"10.0.0.5": "users",
	}

	var out bytes.Buffer
	display.PrintTree(&out, records, svcMap, "abc-123")

	output := out.String()

	// Verify header line.
	assertContains(t, output, "GET /api/orders (trace: abc-123)")

	// Verify tree structure contains expected entries.
	assertContains(t, output, "gateway")
	assertContains(t, output, "auth")
	assertContains(t, output, "Auth/Verify")
	assertContains(t, output, "OK")
	assertContains(t, output, "orders")
	assertContains(t, output, "users")
	assertContains(t, output, "/users/42")

	// Verify final response line.
	assertContains(t, output, "response: 500")
}

func TestPrintTree_NoRecords(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	display.PrintTree(&out, nil, docker.ServiceMap{}, "missing")

	assertContains(t, out.String(), "no records found for trace missing")
}

func TestPrintTree_FiltersbyTraceID(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	records := []model.Record{
		{
			Timestamp: base, Proto: model.ProtoHTTP1,
			SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
			Method: "GET", Path: "/a",
			Status: 200, DurationMs: 10, TraceID: "trace-1",
		},
		{
			Timestamp: base, Proto: model.ProtoHTTP1,
			SrcIP: "10.0.0.3", DstIP: "10.0.0.4",
			Method: "GET", Path: "/b",
			Status: 200, DurationMs: 5, TraceID: "trace-2",
		},
	}

	var out bytes.Buffer
	display.PrintTree(&out, records, docker.ServiceMap{}, "trace-1")

	output := out.String()
	assertContains(t, output, "/a")
	if strings.Contains(output, "/b") {
		t.Error("should not contain records from trace-2")
	}
}
