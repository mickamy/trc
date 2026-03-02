package tui_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mickamy/trc/internal/display/tui"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

func TestStartReader_BasicRead(t *testing.T) {
	t.Parallel()

	records := []model.Record{
		{
			Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Proto:     model.ProtoHTTP1, SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
			Method: "GET", Path: "/a", Status: 200, DurationMs: 1,
		},
		{
			Timestamp: time.Date(2025, 1, 1, 12, 0, 1, 0, time.UTC),
			Proto:     model.ProtoGRPC, SrcIP: "10.0.0.1", DstIP: "10.0.0.3",
			Method: "Auth/Verify", Path: "/auth.Auth/Verify",
			Status: 200, GRPCStatus: "OK", DurationMs: 2,
		},
	}

	svcMap := docker.ServiceMap{
		"10.0.0.1": "gateway",
		"10.0.0.2": "users",
		"10.0.0.3": "auth",
	}

	ch := tui.StartReader(
		t.Context(), strings.NewReader(toNDJSON(t, records)),
		func() docker.ServiceMap { return svcMap }, "",
	)

	got := make([]model.Record, 0, len(records))
	for rec := range ch {
		got = append(got, rec)
	}

	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	if got[0].SrcName != "gateway" || got[0].DstName != "users" {
		t.Errorf("record 0: src=%q dst=%q, want gateway/users", got[0].SrcName, got[0].DstName)
	}
	if got[1].SrcName != "gateway" || got[1].DstName != "auth" {
		t.Errorf("record 1: src=%q dst=%q, want gateway/auth", got[1].SrcName, got[1].DstName)
	}
}

func TestStartReader_Filter(t *testing.T) {
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

	ch := tui.StartReader(
		t.Context(), strings.NewReader(toNDJSON(t, records)),
		func() docker.ServiceMap { return svcMap }, "users",
	)

	got := make([]model.Record, 0, 1)
	for rec := range ch {
		got = append(got, rec)
	}

	if len(got) != 1 {
		t.Fatalf("got %d records, want 1 (filtered)", len(got))
	}
	if got[0].DstName != "users" {
		t.Errorf("record dst=%q, want users", got[0].DstName)
	}
}

func TestStartReader_InvalidJSON(t *testing.T) {
	t.Parallel()

	valid := model.Record{
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Proto:     model.ProtoHTTP1, SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
		Method: "GET", Path: "/ok", Status: 200, DurationMs: 1,
	}
	input := "not json\n" + toNDJSON(t, []model.Record{valid})

	ch := tui.StartReader(
		t.Context(), strings.NewReader(input),
		func() docker.ServiceMap { return docker.ServiceMap{} }, "",
	)

	got := make([]model.Record, 0, 1)
	for rec := range ch {
		got = append(got, rec)
	}

	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
}

func TestWaitForRecord_Record(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Record, 1)
	rec := model.Record{Method: "GET", Path: "/test"}
	ch <- rec

	cmd := tui.WaitForRecord(ch)
	msg := cmd()

	rm, ok := msg.(tui.RecordMsg)
	if !ok {
		t.Fatalf("got %T, want RecordMsg", msg)
	}
	if rm.Record.Path != "/test" {
		t.Errorf("path=%q, want /test", rm.Record.Path)
	}
}

func TestWaitForRecord_EOF(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Record)
	close(ch)

	cmd := tui.WaitForRecord(ch)
	msg := cmd()

	if _, ok := msg.(tui.EOFMsg); !ok {
		t.Fatalf("got %T, want EOFMsg", msg)
	}
}

func toNDJSON(t *testing.T, records []model.Record) string {
	t.Helper()
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("encoding record: %v", err)
		}
	}
	return buf.String()
}
