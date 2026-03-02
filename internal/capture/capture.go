package capture

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"

	"github.com/mickamy/trc/internal/model"
)

// Config holds the settings for a capture session.
type Config struct {
	Interface string
	BPFFilter string
	SnapLen   int32
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Interface: "eth0",
		BPFFilter: "tcp",
		SnapLen:   65535,
	}
}

// Run opens a live packet capture on the configured interface and emits
// parsed HTTP/gRPC records as NDJSON to w. It blocks until ctx is cancelled.
func Run(ctx context.Context, cfg Config, w io.Writer) error {
	handle, err := pcap.OpenLive(cfg.Interface, cfg.SnapLen, true, pcap.BlockForever)
	if err != nil {
		return fmt.Errorf("open capture on %s: %w", cfg.Interface, err)
	}
	defer handle.Close()

	if err := handle.SetBPFFilter(cfg.BPFFilter); err != nil {
		return fmt.Errorf("set BPF filter %q: %w", cfg.BPFFilter, err)
	}

	emitter := NewEmitter(w)
	factory := newStreamFactory(func(r model.Record) {
		if err := emitter.Emit(r); err != nil {
			// Best-effort: log to stderr would be ideal, but we don't
			// want to couple capture to a logger. Drop silently.
			_ = err
		}
	})

	pool := tcpassembly.NewStreamPool(factory)
	assembler := tcpassembly.NewAssembler(pool)

	src := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := src.Packets()

	flushTicker := time.NewTicker(30 * time.Second) //nolint:mnd // periodic flush interval
	defer flushTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			assembler.FlushAll()
			return nil
		case pkt, ok := <-packets:
			if !ok {
				assembler.FlushAll()
				return nil
			}
			tcp, _ := pkt.TransportLayer().(*layers.TCP)
			if tcp == nil {
				continue
			}
			assembler.AssembleWithTimestamp(
				pkt.NetworkLayer().NetworkFlow(),
				tcp,
				pkt.Metadata().Timestamp,
			)
		case <-flushTicker.C:
			assembler.FlushOlderThan(time.Now().Add(-30 * time.Second)) //nolint:mnd // match flush interval
		}
	}
}
