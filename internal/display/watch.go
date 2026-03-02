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

// StreamWatch reads NDJSON records from r and writes formatted output to w.
// It resolves IPs to service names using svcMap and optionally filters by
// service name. It blocks until ctx is cancelled or r is exhausted.
func StreamWatch(
	ctx context.Context, r io.Reader,
	svcMap docker.ServiceMap, filter string, w io.Writer,
) error {
	scanner := bufio.NewScanner(r)
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

	dur := fmt.Sprintf("%.0fms", r.DurationMs)

	return fmt.Sprintf("%-12s %-5s  %-12s → %-12s  %-4s %-20s  %s  %s",
		ts, string(r.Proto), src, dst, r.Method, r.Path, status, dur,
	)
}

func formatStatus(r model.Record) string {
	if r.Proto == model.ProtoGRPC && r.GRPCStatus != "" {
		return r.GRPCStatus
	}
	return strconv.Itoa(r.Status)
}
