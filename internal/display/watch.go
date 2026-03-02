package display

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

// ServiceResolver returns the current service map snapshot.
// It is called for each record so that updates are reflected in real time.
type ServiceResolver func() docker.ServiceMap

// StreamWatch reads NDJSON records from r and writes formatted output to w.
// It resolves IPs to service names using resolve and optionally filters by
// service name. It blocks until ctx is cancelled or r is exhausted.
//
// If r implements io.Closer it will be closed when ctx is cancelled so that
// a blocking read is interrupted promptly.
func StreamWatch(
	ctx context.Context, r io.Reader,
	resolve ServiceResolver, filter string, w io.Writer,
) error {
	// When ctx is cancelled, close the reader (if possible) so that
	// scanner.Scan() unblocks instead of hanging on an idle stream.
	// The done channel prevents the goroutine from leaking when the
	// reader reaches EOF before ctx is cancelled.
	done := make(chan struct{})
	defer close(done)
	if rc, ok := r.(io.Closer); ok {
		go func() {
			select {
			case <-ctx.Done():
				_ = rc.Close()
			case <-done:
			}
		}()
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) //nolint:mnd // 10 MiB max token size
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var rec model.Record
		if err := rec.UnmarshalNDJSON(scanner.Bytes()); err != nil {
			continue
		}

		svcMap := resolve()
		rec.SrcName = ResolveName(svcMap, rec.SrcIP)
		rec.DstName = ResolveName(svcMap, rec.DstIP)

		if filter != "" && !MatchesFilter(rec, filter) {
			continue
		}

		fmt.Fprintln(w, FormatRecord(rec))
	}
	if err := scanner.Err(); err != nil {
		// If the context was cancelled the reader was closed, causing a
		// read error that we can safely ignore.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("reading records: %w", err)
	}
	return nil
}

func ResolveName(svcMap docker.ServiceMap, ip string) string {
	if name, ok := svcMap[ip]; ok {
		return name
	}
	return ip
}

func MatchesFilter(rec model.Record, filter string) bool {
	f := strings.ToLower(filter)
	return strings.Contains(strings.ToLower(rec.SrcName), f) ||
		strings.Contains(strings.ToLower(rec.DstName), f)
}

func FormatRecord(r model.Record) string {
	ts := r.Timestamp.Format("15:04:05.000")

	src := r.SrcName
	dst := r.DstName

	status := formatStatus(r)

	dur := formatDuration(r.DurationMs)

	methodPath := r.Method + " " + r.Path
	if r.Proto == model.ProtoGRPC {
		// For gRPC, Method already contains "Service/Method" — Path is redundant.
		methodPath = r.Method
	}

	line := fmt.Sprintf("%-12s %-5s  %-12s → %-12s  %-28s %4s  %s",
		ts, string(r.Proto), src, dst, methodPath, status, dur,
	)
	if r.TraceID != "" {
		line += "  " + r.TraceID
	}
	return line
}

func formatDuration(ms float64) string {
	if ms >= 1 {
		return fmt.Sprintf("%dms", int(ms))
	}
	us := ms * 1000 //nolint:mnd // ms to µs
	return fmt.Sprintf("%dµs", int(us))
}

func formatStatus(r model.Record) string {
	if r.Proto == model.ProtoGRPC && r.GRPCStatus != "" {
		return r.GRPCStatus
	}
	return strconv.Itoa(r.Status)
}
