package capture

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/mickamy/trc/internal/model"
)

// Emitter writes Records as NDJSON to an io.Writer.
// It is safe for concurrent use.
type Emitter struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// NewEmitter creates an Emitter that writes to w.
func NewEmitter(w io.Writer) *Emitter {
	return &Emitter{enc: json.NewEncoder(w)}
}

// Emit serializes r as a single JSON line.
func (e *Emitter) Emit(r model.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enc.Encode(r)
}
