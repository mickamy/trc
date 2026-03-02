package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	var b strings.Builder

	if len(m.records) == 0 {
		b.WriteString("Waiting for traffic...\n")
	} else {
		// Calculate visible window.
		contentHeight := m.height - 2 // 1 for footer, 1 for safety
		if contentHeight < 1 {
			contentHeight = 10 //nolint:mnd // fallback
		}

		start := 0
		end := len(m.records)
		if end-start > contentHeight {
			// Center the cursor in the window.
			half := contentHeight / 2
			start = max(m.cursor-half, 0)
			end = start + contentHeight
			if end > len(m.records) {
				end = len(m.records)
				start = end - contentHeight
			}
		}

		for i := start; i < end; i++ {
			if i == m.cursor {
				b.WriteString("\x1b[1m▶ ")
				b.WriteString(display.FormatRecord(m.records[i]))
				b.WriteString("\x1b[0m\n")
			} else {
				b.WriteString("  ")
				b.WriteString(display.FormatRecord(m.records[i]))
				b.WriteByte('\n')
			}
		}
	}

	// Footer
	b.WriteByte('\n')
	switch {
	case m.statusMsg != "":
		b.WriteString(m.statusMsg)
	case m.eof:
		b.WriteString(" ↑/↓: move  Enter: tree  m: map  e: export  q: quit  (EOF)")
	default:
		b.WriteString(" ↑/↓: move  Enter: tree  m: map  e: export  q: quit")
	}

	return b.String()
}

func (m Model) viewTree() string {
	return m.treeOutput + "\n Esc: back  q: quit"
}

func (m Model) viewMap() string {
	return m.mapOutput + "\n Esc: back  q: quit"
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
