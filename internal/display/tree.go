package display

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

// PrintTree renders a request chain as an ASCII tree for the given trace ID.
func PrintTree(
	w io.Writer, records []model.Record,
	svcMap docker.ServiceMap, traceID string,
) {
	var filtered []model.Record
	for _, r := range records {
		if r.TraceID == traceID {
			r.SrcName = resolveName(svcMap, r.SrcIP)
			r.DstName = resolveName(svcMap, r.DstIP)
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		fmt.Fprintf(w, "no records found for trace %s\n", traceID)
		return
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	root := filtered[0]
	children := buildChildren(filtered)

	fmt.Fprintf(w, "%s %s (trace: %s)  total: %.0fms\n",
		root.Method, root.Path, traceID, totalDuration(filtered))

	childIdxs := children[0]
	for i, childIdx := range childIdxs {
		isLast := i == len(childIdxs)-1
		printNode(w, filtered, children, childIdx, "", isLast)
	}

	fmt.Fprintf(w, "└─ response: %s\n", formatTreeStatus(root))
}

func buildChildren(records []model.Record) map[int][]int {
	children := make(map[int][]int)
	for i := 1; i < len(records); i++ {
		parentIdx := findParent(records, i)
		children[parentIdx] = append(children[parentIdx], i)
	}
	return children
}

func findParent(records []model.Record, idx int) int {
	r := records[idx]
	for i := idx - 1; i >= 0; i-- {
		if records[i].DstIP == r.SrcIP &&
			records[i].Timestamp.Before(r.Timestamp) {
			return i
		}
	}
	return 0
}

func printNode(
	w io.Writer, records []model.Record,
	children map[int][]int, idx int, prefix string, isLast bool,
) {
	r := records[idx]

	connector := "├─"
	if isLast {
		connector = "└─"
	}

	label := r.Method
	if r.Path != "" && r.Proto != model.ProtoGRPC {
		label = r.Method + " " + r.Path
	}

	fmt.Fprintf(w, "%s%s %s → %-10s %-5s %-20s %s  %.0fms\n",
		prefix, connector,
		r.SrcName, r.DstName,
		string(r.Proto), label,
		formatTreeStatus(r), r.DurationMs,
	)

	childIdxs := children[idx]
	nextPrefix := prefix + "│  "
	if isLast {
		nextPrefix = prefix + "   "
	}

	for i, childIdx := range childIdxs {
		printNode(w, records, children, childIdx, nextPrefix, i == len(childIdxs)-1)
	}
}

func totalDuration(records []model.Record) float64 {
	if len(records) == 0 {
		return 0
	}
	first := records[0].Timestamp
	var maxEnd float64
	for _, r := range records {
		end := float64(r.Timestamp.Sub(first).Milliseconds()) + r.DurationMs
		if end > maxEnd {
			maxEnd = end
		}
	}
	return maxEnd
}

func formatTreeStatus(r model.Record) string {
	if r.Proto == model.ProtoGRPC && r.GRPCStatus != "" {
		return r.GRPCStatus
	}
	return strconv.Itoa(r.Status)
}
