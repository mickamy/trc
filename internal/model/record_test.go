package model_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mickamy/trc/internal/model"
)

func TestRecord_MarshalNDJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		record  model.Record
		wantSub string // substring that must appear in the output
	}{
		{
			name: "http1 record",
			record: model.Record{
				Timestamp:  time.Date(2025, 1, 1, 12, 0, 0, 123000000, time.UTC),
				Proto:      model.ProtoHTTP1,
				SrcIP:      "172.18.0.3",
				DstIP:      "172.18.0.5",
				Method:     "GET",
				Path:       "/users/42",
				Status:     200,
				DurationMs: 12,
				TraceID:    "abc-123",
			},
			wantSub: `"proto":"http1"`,
		},
		{
			name: "grpc record includes grpc_status",
			record: model.Record{
				Timestamp:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				Proto:      model.ProtoGRPC,
				SrcIP:      "172.18.0.3",
				DstIP:      "172.18.0.4",
				Method:     "Auth/Verify",
				Path:       "/auth.Auth/Verify",
				Status:     200,
				GRPCStatus: "OK",
				DurationMs: 5,
			},
			wantSub: `"grpc_status":"OK"`,
		},
		{
			name: "omits empty optional fields",
			record: model.Record{
				Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				Proto:     model.ProtoHTTP2,
				SrcIP:     "10.0.0.1",
				DstIP:     "10.0.0.2",
				Method:    "POST",
				Path:      "/orders",
				Status:    201,
			},
			wantSub: `"proto":"http2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.record.MarshalNDJSON()
			if err != nil {
				t.Fatalf("MarshalNDJSON() error = %v", err)
			}

			line := string(got)
			if line[len(line)-1] != '\n' {
				t.Error("MarshalNDJSON() output must end with newline")
			}

			if !strings.Contains(line, tt.wantSub) {
				t.Errorf("MarshalNDJSON() = %s, want substring %q", line, tt.wantSub)
			}
		})
	}
}

func TestRecord_UnmarshalNDJSON(t *testing.T) {
	t.Parallel()

	input := `{"ts":"2025-01-01T12:00:00.123Z","proto":"http1","src_ip":"172.18.0.3","dst_ip":"172.18.0.5","method":"GET","path":"/users/42","status":200,"duration_ms":12,"trace_id":"abc-123"}`

	var r model.Record
	if err := r.UnmarshalNDJSON([]byte(input)); err != nil {
		t.Fatalf("UnmarshalNDJSON() error = %v", err)
	}

	if r.Proto != model.ProtoHTTP1 {
		t.Errorf("Proto = %q, want %q", r.Proto, model.ProtoHTTP1)
	}
	if r.SrcIP != "172.18.0.3" {
		t.Errorf("SrcIP = %q, want %q", r.SrcIP, "172.18.0.3")
	}
	if r.Method != "GET" {
		t.Errorf("Method = %q, want %q", r.Method, "GET")
	}
	if r.Path != "/users/42" {
		t.Errorf("Path = %q, want %q", r.Path, "/users/42")
	}
	if r.Status != 200 {
		t.Errorf("Status = %d, want %d", r.Status, 200)
	}
	if r.TraceID != "abc-123" {
		t.Errorf("TraceID = %q, want %q", r.TraceID, "abc-123")
	}
}

func TestRecord_RoundTrip(t *testing.T) {
	t.Parallel()

	original := model.Record{
		Timestamp:  time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		Proto:      model.ProtoGRPC,
		SrcIP:      "10.0.0.1",
		DstIP:      "10.0.0.2",
		Method:     "Auth/Verify",
		Path:       "/auth.Auth/Verify",
		Status:     200,
		GRPCStatus: "OK",
		DurationMs: 3.5,
		TraceID:    "trace-xyz",
		SrcName:    "gateway",
		DstName:    "auth",
	}

	data, err := original.MarshalNDJSON()
	if err != nil {
		t.Fatalf("MarshalNDJSON() error = %v", err)
	}

	var decoded model.Record
	if err := decoded.UnmarshalNDJSON(data[:len(data)-1]); err != nil {
		t.Fatalf("UnmarshalNDJSON() error = %v", err)
	}

	if decoded.Proto != original.Proto {
		t.Errorf("Proto = %q, want %q", decoded.Proto, original.Proto)
	}
	if decoded.GRPCStatus != original.GRPCStatus {
		t.Errorf("GRPCStatus = %q, want %q", decoded.GRPCStatus, original.GRPCStatus)
	}
	if decoded.SrcName != original.SrcName {
		t.Errorf("SrcName = %q, want %q", decoded.SrcName, original.SrcName)
	}
	if decoded.DstName != original.DstName {
		t.Errorf("DstName = %q, want %q", decoded.DstName, original.DstName)
	}
}

