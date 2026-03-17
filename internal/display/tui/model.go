package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mickamy/trc/internal/display"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

type view int

const (
	viewWatch view = iota
	viewTree
	viewMap
)

// Model is the bubbletea model for the TUI.
type Model struct {
	currentView view
	records     []model.Record
	cursor      int
	autoScroll  bool
	treeOutput  string
	mapOutput   string
	recordCh    <-chan model.Record
	svcMapFn    func() docker.ServiceMap
	width       int
	height      int
	eof         bool
	statusMsg   string
}

func newModel(ch <-chan model.Record, svcMapFn func() docker.ServiceMap) Model {
	return Model{
		currentView: viewWatch,
		autoScroll:  true,
		recordCh:    ch,
		svcMapFn:    svcMapFn,
	}
}

// Init starts listening for the first record.
func (m Model) Init() tea.Cmd {
	return waitForRecord(m.recordCh)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case RecordMsg:
		m.records = append(m.records, msg.Record)
		if m.autoScroll {
			m.cursor = len(m.records) - 1
		}
		m.statusMsg = ""
		return m, waitForRecord(m.recordCh)

	case EOFMsg:
		m.eof = true
		return m, nil

	case ErrMsg:
		m.statusMsg = fmt.Sprintf("error: %v", msg.Err)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// View renders the current view.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	switch m.currentView {
	case viewWatch:
		return m.viewWatch()
	case viewTree:
		return m.viewTree()
	case viewMap:
		return m.viewMap()
	}
	return m.viewWatch()
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear status on any key press (except the key that sets it).
	m.statusMsg = ""

	switch m.currentView {
	case viewWatch:
		return m.handleWatchKey(msg)
	case viewTree, viewMap:
		return m.handleDetailKey(msg)
	}
	return m, nil
}

func (m Model) handleWatchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.autoScroll = false
		}
	case "down", "j":
		if m.cursor < len(m.records)-1 {
			m.cursor++
		}
		if m.cursor == len(m.records)-1 {
			m.autoScroll = true
		}
	case "enter":
		if len(m.records) > 0 {
			rec := m.records[m.cursor]
			if rec.TraceID != "" {
				var buf bytes.Buffer
				display.PrintTree(&buf, m.records, m.svcMapFn(), rec.TraceID)
				m.treeOutput = buf.String()
				m.currentView = viewTree
			}
		}
	case "m":
		if len(m.records) > 0 {
			var buf bytes.Buffer
			display.PrintMap(&buf, m.records, m.svcMapFn())
			m.mapOutput = buf.String()
			m.currentView = viewMap
		}
	case "e":
		if len(m.records) > 0 {
			filename := fmt.Sprintf("trc-watch-%s.ndjson", time.Now().Format("20060102T150405"))
			if err := exportRecords(filename, m.records); err != nil {
				m.statusMsg = fmt.Sprintf("export failed: %v", err)
			} else {
				m.statusMsg = fmt.Sprintf("Exported %d records to %s", len(m.records), filename)
			}
		}
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.currentView = viewWatch
	}
	return m, nil
}

func (m Model) viewWatch() string {
	innerWidth := max(m.width-4, 20) //nolint:mnd // 4 = border (2) + margin (2)

	if len(m.records) == 0 {
		return "Waiting for traffic..."
	}

	title := fmt.Sprintf(" trc (%d records) ", len(m.records))
	colRequest := max(innerWidth-colFixedWatch, 10) //nolint:mnd // minimum column width

	// Overhead: border top (1) + header (1) + border bottom (1) + footer (1).
	dataRows := max(m.height-4, 1) //nolint:mnd // 4 lines of overhead

	start := 0
	end := len(m.records)
	if end-start > dataRows {
		half := dataRows / 2
		start = max(m.cursor-half, 0)
		end = start + dataRows
		if end > len(m.records) {
			end = len(m.records)
			start = end - dataRows
		}
	}

	header := fmt.Sprintf("  %-*s %-*s %-*s → %-*s %-*s %*s %*s",
		colTime, "Time",
		colProto, "Proto",
		colSrc, "Src",
		colDst, "Dst",
		colRequest, "Request",
		colStatus, "Stat",
		colDuration, "Duration",
	)

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Bold(true).Render(header))
	for i := start; i < end; i++ {
		rows = append(rows, m.renderWatchRow(m.records[i], i == m.cursor, colRequest))
	}

	content := strings.Join(rows, "\n")

	var footer string
	switch {
	case m.statusMsg != "":
		footer = "  " + m.statusMsg
	case m.eof:
		footer = "  ↑/↓: move  Enter: tree  m: map  e: export  q: quit  (EOF)"
	default:
		footer = "  ↑/↓: move  Enter: tree  m: map  e: export  q: quit"
	}

	return renderBorderedBox(content, title, innerWidth) + "\n" + footer
}

func (m Model) renderWatchRow(r model.Record, isCursor bool, colRequest int) string {
	marker := "  "
	if isCursor {
		marker = "▶ "
	}

	ts := r.Timestamp.Format("15:04:05.000")
	proto := truncateStr(string(r.Proto), colProto)
	src := truncateStr(r.SrcName, colSrc)
	dst := truncateStr(r.DstName, colDst)

	methodPath := r.Method + " " + r.Path
	if r.Proto == model.ProtoGRPC {
		methodPath = r.Method
	}
	methodPath = truncateStr(methodPath, colRequest)

	status := truncateStr(formatStat(string(r.Proto), r.Status, r.GRPCStatus), colStatus)
	dur := truncateStr(formatDur(r.DurationMs), colDuration)

	row := fmt.Sprintf("%s%-*s %-*s %-*s → %-*s %-*s %*s %*s",
		marker,
		colTime, ts,
		colProto, proto,
		colSrc, src,
		colDst, dst,
		colRequest, methodPath,
		colStatus, status,
		colDuration, dur,
	)

	if isCursor {
		return lipgloss.NewStyle().Bold(true).Render(row)
	}
	return row
}

func (m Model) viewTree() string {
	innerWidth := max(m.width-4, 20) //nolint:mnd // 4 = border + margin
	content := strings.TrimRight(m.treeOutput, "\n ")
	return renderBorderedBoxWithHelp(
		content, " Trace ", " Esc: back  q: quit ", innerWidth,
	)
}

func (m Model) viewMap() string {
	innerWidth := max(m.width-4, 20) //nolint:mnd // 4 = border + margin
	content := strings.TrimRight(m.mapOutput, "\n ")
	return renderBorderedBoxWithHelp(
		content, " Service Dependencies ", " Esc: back  q: quit ", innerWidth,
	)
}

// exportRecords writes all records as NDJSON to the named file.
func exportRecords(filename string, records []model.Record) error {
	f, err := os.Create(filename) //nolint:gosec // filename is generated internally, not from user input
	if err != nil {
		return fmt.Errorf("creating export file: %w", err)
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	for _, rec := range records {
		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("encoding record: %w", err)
		}
	}
	return nil
}
