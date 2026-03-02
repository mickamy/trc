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
func StreamWatch(
	ctx context.Context, r io.Reader,
	resolve ServiceResolver, filter string, w io.Writer,
) error {
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
		rec.SrcName = resolveName(svcMap, rec.SrcIP)
		rec.DstName = resolveName(svcMap, rec.DstIP)

		if filter != "" && !matchesFilter(rec, filter) {
			continue
		}

		fmt.Fprintln(w, formatRecord(rec))
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading records: %w", err)
	}
	return nil
}

func resolveName(svcMap docker.ServiceMap, ip string) string {
	if name, ok := svcMap[ip]; ok {
		return name
	}
	return ip
}

func matchesFilter(rec model.Record, filter string) bool {
	f := strings.ToLower(filter)
	return strings.Contains(strings.ToLower(rec.SrcName), f) ||
		strings.Contains(strings.ToLower(rec.DstName), f)
}

func formatRecord(r model.Record) string {
	ts := r.Timestamp.Format("15:04:05.000")

	src := r.SrcName
	dst := r.DstName

	status := formatStatus(r)

	dur := fmt.Sprintf("%dms", int(r.DurationMs))

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

func formatStatus(r model.Record) string {
	if r.Proto == model.ProtoGRPC && r.GRPCStatus != "" {
		return r.GRPCStatus
	}
	return strconv.Itoa(r.Status)
}
