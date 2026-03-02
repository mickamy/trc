package tui

import "github.com/mickamy/trc/internal/model"

// RecordMsg is sent when a new record is read from the capture stream.
type RecordMsg struct{ Record model.Record }

// EOFMsg is sent when the capture stream ends.
type EOFMsg struct{}

// ErrMsg is sent when an error occurs reading the capture stream.
type ErrMsg struct{ Err error }
