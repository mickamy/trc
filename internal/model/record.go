package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// Protocol represents the application-layer protocol detected on a TCP stream.
type Protocol string

const (
	ProtoHTTP1 Protocol = "http1"
	ProtoHTTP2 Protocol = "http2"
	ProtoGRPC  Protocol = "grpc"
)

// Record is a single request/response pair captured from the network.
// trcd emits these as NDJSON; the CLI reads and enriches them with service names.
type Record struct {
	Timestamp  time.Time `json:"ts"`
	Proto      Protocol  `json:"proto"`
	SrcIP      string    `json:"src_ip"`
	DstIP      string    `json:"dst_ip"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	GRPCStatus string    `json:"grpc_status,omitempty"`
	DurationMs float64   `json:"duration_ms"`
	TraceID    string    `json:"trace_id,omitempty"`

	// SrcName and DstName are resolved by the CLI, not set by trcd.
	SrcName string `json:"src_name,omitempty"`
	DstName string `json:"dst_name,omitempty"`
}

// MarshalNDJSON encodes r as a single JSON line followed by a newline.
func (r Record) MarshalNDJSON() ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("marshaling record: %w", err)
	}
	return append(b, '\n'), nil
}

// UnmarshalNDJSON decodes a single JSON line into r.
func (r *Record) UnmarshalNDJSON(line []byte) error {
	if err := json.Unmarshal(line, r); err != nil {
		return fmt.Errorf("unmarshaling record: %w", err)
	}
	return nil
}
