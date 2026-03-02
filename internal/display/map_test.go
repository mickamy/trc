package display_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/mickamy/trc/internal/display"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

func TestPrintMap_BasicAggregation(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	records := []model.Record{
		{
			Timestamp: base, Proto: model.ProtoGRPC,
			SrcIP: "10.0.0.2", DstIP: "10.0.0.3",
			Method: "Auth/Verify", Status: 200,
			GRPCStatus: "OK", DurationMs: 3,
		},
		{
			Timestamp: base, Proto: model.ProtoGRPC,
			SrcIP: "10.0.0.2", DstIP: "10.0.0.3",
			Method: "Auth/Verify", Status: 200,
			GRPCStatus: "OK", DurationMs: 5,
		},
		{
			Timestamp: base, Proto: model.ProtoHTTP1,
			SrcIP: "10.0.0.2", DstIP: "10.0.0.5",
			Method: "GET", Path: "/users/42",
			Status: 200, DurationMs: 15,
		},
		{
			Timestamp: base, Proto: model.ProtoHTTP1,
			SrcIP: "10.0.0.2", DstIP: "10.0.0.5",
			Method: "GET", Path: "/users/1",
			Status: 500, DurationMs: 10,
		},
		{
			Timestamp: base, Proto: model.ProtoHTTP2,
			SrcIP: "10.0.0.2", DstIP: "10.0.0.4",
			Method: "POST", Path: "/orders",
			Status: 201, DurationMs: 38,
		},
	}

	svcMap := docker.ServiceMap{
		"10.0.0.2": "gateway",
		"10.0.0.3": "auth",
		"10.0.0.4": "orders",
		"10.0.0.5": "users",
	}

	var out bytes.Buffer
	display.PrintMap(&out, records, svcMap)

	output := out.String()

	assertContains(t, output, "Service Dependencies (5 requests)")
	assertContains(t, output, "gateway")
	assertContains(t, output, "→ auth")
	assertContains(t, output, "grpc")
	assertContains(t, output, "2 req")
	assertContains(t, output, "→ users")
	assertContains(t, output, "1 errors")
	assertContains(t, output, "→ orders")
}

func TestPrintMap_GRPCErrors(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	records := []model.Record{
		{
			Timestamp: base, Proto: model.ProtoGRPC,
			SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
			Method: "Auth/Verify", Status: 200,
			GRPCStatus: "UNAUTHENTICATED", DurationMs: 2,
		},
		{
			Timestamp: base, Proto: model.ProtoGRPC,
			SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
			Method: "Auth/Verify", Status: 200,
			GRPCStatus: "OK", DurationMs: 4,
		},
	}

	var out bytes.Buffer
	display.PrintMap(&out, records, docker.ServiceMap{})

	output := out.String()
	assertContains(t, output, "1 errors")
}

func TestPrintMap_NoRecords(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	display.PrintMap(&out, nil, docker.ServiceMap{})

	assertContains(t, out.String(), "no records captured")
}
