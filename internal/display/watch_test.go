package display_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mickamy/trc/internal/display"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

func TestStreamWatch_BasicOutput(t *testing.T) {
	t.Parallel()

	records := []model.Record{
		{
			Timestamp: time.Date(2025, 1, 1, 12, 1, 3, 421000000, time.UTC),
			Proto:     model.ProtoHTTP1, SrcIP: "172.18.0.3", DstIP: "172.18.0.5",
			Method: "GET", Path: "/users/42", Status: 200, DurationMs: 12,
		},
		{
			Timestamp: time.Date(2025, 1, 1, 12, 1, 3, 430000000, time.UTC),
			Proto:     model.ProtoHTTP2, SrcIP: "172.18.0.3", DstIP: "172.18.0.6",
			Method: "POST", Path: "/orders", Status: 201, DurationMs: 23,
		},
		{
			Timestamp:  time.Date(2025, 1, 1, 12, 1, 3, 445000000, time.UTC),
			Proto:      model.ProtoGRPC,
			SrcIP:      "172.18.0.3",
			DstIP:      "172.18.0.4",
			Method:     "Auth/Verify",
			Path:       "/auth.Auth/Verify",
			Status:     200,
			GRPCStatus: "OK",
			DurationMs: 5,
		},
	}

	svcMap := docker.ServiceMap{
		"172.18.0.3": "gateway",
		"172.18.0.4": "auth",
		"172.18.0.5": "users",
		"172.18.0.6": "orders",
	}

	var out bytes.Buffer
	err := display.StreamWatch(
		t.Context(), strings.NewReader(toNDJSON(t, records)),
		func() docker.ServiceMap { return svcMap }, "", &out,
	)
	if err != nil {
		t.Fatalf("StreamWatch() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	assertContains(t, lines[0], "gateway")
	assertContains(t, lines[0], "users")
	assertContains(t, lines[0], "GET")
	assertContains(t, lines[0], "/users/42")
	assertContains(t, lines[0], "200")

	assertContains(t, lines[1], "http2")
	assertContains(t, lines[1], "POST")

	assertContains(t, lines[2], "grpc")
	assertContains(t, lines[2], "Auth/Verify")
	assertContains(t, lines[2], "OK")
}

func TestStreamWatch_Filter(t *testing.T) {
	t.Parallel()

	records := []model.Record{
		{
			Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Proto:     model.ProtoHTTP1, SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
			Method: "GET", Path: "/a", Status: 200, DurationMs: 1,
		},
		{
			Timestamp: time.Date(2025, 1, 1, 12, 0, 1, 0, time.UTC),
			Proto:     model.ProtoHTTP1, SrcIP: "10.0.0.3", DstIP: "10.0.0.4",
			Method: "GET", Path: "/b", Status: 200, DurationMs: 2,
		},
	}

	svcMap := docker.ServiceMap{
		"10.0.0.1": "gateway",
		"10.0.0.2": "users",
		"10.0.0.3": "orders",
		"10.0.0.4": "auth",
	}

	var out bytes.Buffer
	err := display.StreamWatch(
		t.Context(), strings.NewReader(toNDJSON(t, records)),
		func() docker.ServiceMap { return svcMap }, "users", &out,
	)
	if err != nil {
		t.Fatalf("StreamWatch() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (filtered)", len(lines))
	}
	assertContains(t, lines[0], "users")
}

func TestStreamWatch_UnknownIP(t *testing.T) {
	t.Parallel()

	records := []model.Record{
		{
			Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Proto:     model.ProtoHTTP1, SrcIP: "10.0.0.99", DstIP: "10.0.0.100",
			Method: "GET", Path: "/x", Status: 200, DurationMs: 1,
		},
	}

	var out bytes.Buffer
	err := display.StreamWatch(
		t.Context(), strings.NewReader(toNDJSON(t, records)),
		func() docker.ServiceMap { return docker.ServiceMap{} }, "", &out,
	)
	if err != nil {
		t.Fatalf("StreamWatch() error = %v", err)
	}

	assertContains(t, out.String(), "10.0.0.99")
	assertContains(t, out.String(), "10.0.0.100")
}

func TestStreamWatch_InvalidJSON(t *testing.T) {
	t.Parallel()

	valid := model.Record{
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Proto:     model.ProtoHTTP1, SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
		Method: "GET", Path: "/ok", Status: 200, DurationMs: 1,
	}
	input := "not json\n" + toNDJSON(t, []model.Record{valid})

	var out bytes.Buffer
	err := display.StreamWatch(
		t.Context(), strings.NewReader(input),
		func() docker.ServiceMap { return docker.ServiceMap{} }, "", &out,
	)
	if err != nil {
		t.Fatalf("StreamWatch() error = %v", err)
	}

	assertContains(t, out.String(), "/ok")
	if strings.Contains(out.String(), "not json") {
		t.Error("invalid JSON line should have been skipped")
	}
}

func toNDJSON(t *testing.T, records []model.Record) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("encoding record: %v", err)
		}
	}
	return buf.String()
}

func assertContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("output %q does not contain %q", s, sub)
	}
}
