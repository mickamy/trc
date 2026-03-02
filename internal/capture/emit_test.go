package capture_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickamy/trc/internal/capture"
	"github.com/mickamy/trc/internal/model"
)

func TestEmitter_Emit(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	em := capture.NewEmitter(&buf)

	r := model.Record{
		Timestamp:  time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Proto:      model.ProtoHTTP1,
		SrcIP:      "10.0.0.1",
		DstIP:      "10.0.0.2",
		Method:     "GET",
		Path:       "/health",
		Status:     200,
		DurationMs: 1,
	}

	if err := em.Emit(r); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	var decoded model.Record
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Proto != model.ProtoHTTP1 {
		t.Errorf("Proto = %q, want %q", decoded.Proto, model.ProtoHTTP1)
	}
	if decoded.Method != "GET" {
		t.Errorf("Method = %q, want %q", decoded.Method, "GET")
	}
}

func TestEmitter_MultipleLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	em := capture.NewEmitter(&buf)

	for i := range 3 {
		r := model.Record{
			Timestamp: time.Date(2025, 1, 1, 12, 0, i, 0, time.UTC),
			Proto:     model.ProtoHTTP2,
			SrcIP:     "10.0.0.1",
			DstIP:     "10.0.0.2",
			Method:    "POST",
			Path:      "/orders",
			Status:    201,
		}
		if err := em.Emit(r); err != nil {
			t.Fatalf("Emit() error = %v", err)
		}
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}

	for i, line := range lines {
		var decoded model.Record
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Errorf("line %d: Unmarshal() error = %v", i, err)
		}
	}
}

func TestEmitter_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	em := capture.NewEmitter(&buf)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)

	for range n {
		go func() {
			defer wg.Done()
			r := model.Record{
				Timestamp: time.Now(),
				Proto:     model.ProtoGRPC,
				SrcIP:     "10.0.0.1",
				DstIP:     "10.0.0.2",
				Method:    "Auth/Verify",
				Path:      "/auth.Auth/Verify",
				Status:    200,
			}
			if err := em.Emit(r); err != nil {
				t.Errorf("Emit() error = %v", err)
			}
		}()
	}

	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != n {
		t.Errorf("got %d lines, want %d", len(lines), n)
	}
}
