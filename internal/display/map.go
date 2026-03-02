package display

import (
	"fmt"
	"io"
	"sort"

	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

// edgeKey groups records by source, destination, and protocol.
type edgeKey struct {
	src   string
	dst   string
	proto model.Protocol
}

// edgeStats holds aggregated statistics for a single edge.
type edgeStats struct {
	count    int
	totalMs  float64
	errCount int
}

// PrintMap renders an aggregated service dependency summary to w.
func PrintMap(
	w io.Writer, records []model.Record,
	svcMap docker.ServiceMap,
) {
	if len(records) == 0 {
		fmt.Fprintln(w, "no records captured")
		return
	}

	edges := make(map[edgeKey]*edgeStats)
	for _, r := range records {
		src := resolveName(svcMap, r.SrcIP)
		dst := resolveName(svcMap, r.DstIP)
		key := edgeKey{src: src, dst: dst, proto: r.Proto}
		s, ok := edges[key]
		if !ok {
			s = &edgeStats{}
			edges[key] = s
		}
		s.count++
		s.totalMs += r.DurationMs
		if isError(r) {
			s.errCount++
		}
	}

	// Group edges by source service, sorted alphabetically.
	type edge struct {
		key   edgeKey
		stats *edgeStats
	}
	bySrc := make(map[string][]edge)
	for k, s := range edges {
		bySrc[k.src] = append(bySrc[k.src], edge{key: k, stats: s})
	}

	srcs := make([]string, 0, len(bySrc))
	for src := range bySrc {
		srcs = append(srcs, src)
	}
	sort.Strings(srcs)

	fmt.Fprintf(w, "Service Dependencies (%d requests)\n\n", len(records))

	for _, src := range srcs {
		fmt.Fprintln(w, src)

		group := bySrc[src]
		sort.Slice(group, func(i, j int) bool {
			if group[i].key.dst != group[j].key.dst {
				return group[i].key.dst < group[j].key.dst
			}
			return group[i].key.proto < group[j].key.proto
		})

		for _, e := range group {
			avg := e.stats.totalMs / float64(e.stats.count)
			fmt.Fprintf(w, "  → %-16s %-5s  %3d req   avg %3.0fms   %d errors\n",
				e.key.dst, string(e.key.proto),
				e.stats.count, avg, e.stats.errCount,
			)
		}
		fmt.Fprintln(w)
	}
}

// isError reports whether a record represents a failed request.
func isError(r model.Record) bool {
	if r.Proto == model.ProtoGRPC {
		return r.GRPCStatus != "" && r.GRPCStatus != "OK"
	}
	return r.Status >= 400 //nolint:mnd // HTTP error threshold
}
