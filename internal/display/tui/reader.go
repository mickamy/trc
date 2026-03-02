package tui

import (
	"bufio"
	"context"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mickamy/trc/internal/display"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

// startReader launches a goroutine that reads NDJSON records from r,
// resolves service names, applies the filter, and sends matching records
// to the returned channel. The channel is closed when r is exhausted or
// ctx is cancelled.
func startReader(
	ctx context.Context, r io.Reader,
	svcMapFn func() docker.ServiceMap, filter string,
) <-chan model.Record {
	ch := make(chan model.Record)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) //nolint:mnd // 10 MiB max token
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var rec model.Record
			if err := rec.UnmarshalNDJSON(scanner.Bytes()); err != nil {
				continue
			}

			svcMap := svcMapFn()
			rec.SrcName = display.ResolveName(svcMap, rec.SrcIP)
			rec.DstName = display.ResolveName(svcMap, rec.DstIP)

			if filter != "" && !display.MatchesFilter(rec, filter) {
				continue
			}

			select {
			case ch <- rec:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// waitForRecord returns a tea.Cmd that blocks until the next record is
// available on ch, then returns a RecordMsg or EOFMsg.
func waitForRecord(ch <-chan model.Record) tea.Cmd {
	return func() tea.Msg {
		rec, ok := <-ch
		if !ok {
			return EOFMsg{}
		}
		return RecordMsg{Record: rec}
	}
}

// filterString normalises the filter value, returning "" when empty.
func filterString(f string) string {
	return strings.TrimSpace(f)
}
