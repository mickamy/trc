package tui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mickamy/trc/internal/display/tui"
	"github.com/mickamy/trc/internal/model"
)

func sampleRecord(src, dst, traceID string) model.Record {
	return model.Record{
		Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Proto:     model.ProtoHTTP1,
		SrcIP:     "10.0.0.1", DstIP: "10.0.0.2",
		SrcName: src, DstName: dst,
		Method: "GET", Path: "/test",
		Status: 200, DurationMs: 5,
		TraceID: traceID,
	}
}

func mustUpdate(t *testing.T, m tea.Model, msg tea.Msg) tui.Model {
	t.Helper()
	updated, _ := m.Update(msg)
	return tui.AsModel(updated)
}

func TestModel_RecordMsg_AddsRecord(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel(nil)
	um := mustUpdate(t, m, tui.RecordMsg{Record: sampleRecord("gw", "users", "t1")})

	if len(um.Records()) != 1 {
		t.Fatalf("got %d records, want 1", len(um.Records()))
	}
	if um.Cursor() != 0 {
		t.Errorf("cursor=%d, want 0", um.Cursor())
	}
}

func TestModel_RecordMsg_AutoScroll(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{sampleRecord("a", "b", "t1")})
	um := mustUpdate(t, m, tui.RecordMsg{Record: sampleRecord("c", "d", "t2")})

	if um.Cursor() != 1 {
		t.Errorf("cursor=%d, want 1 (auto-scroll)", um.Cursor())
	}
}

func TestModel_CursorMovement(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{
		sampleRecord("a", "b", ""),
		sampleRecord("c", "d", ""),
		sampleRecord("e", "f", ""),
	})

	// Move up.
	um := mustUpdate(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if um.Cursor() != 1 {
		t.Errorf("after up: cursor=%d, want 1", um.Cursor())
	}
	if um.AutoScroll() {
		t.Error("autoScroll should be false after moving up")
	}

	// Move up again.
	um = mustUpdate(t, um, tea.KeyMsg{Type: tea.KeyUp})
	if um.Cursor() != 0 {
		t.Errorf("after up: cursor=%d, want 0", um.Cursor())
	}

	// Move up at top — stays at 0.
	um = mustUpdate(t, um, tea.KeyMsg{Type: tea.KeyUp})
	if um.Cursor() != 0 {
		t.Errorf("at top: cursor=%d, want 0", um.Cursor())
	}

	// Move down to bottom.
	um = mustUpdate(t, um, tea.KeyMsg{Type: tea.KeyDown})
	um = mustUpdate(t, um, tea.KeyMsg{Type: tea.KeyDown})
	if um.Cursor() != 2 {
		t.Errorf("after down: cursor=%d, want 2", um.Cursor())
	}
	if !um.AutoScroll() {
		t.Error("autoScroll should be true at bottom")
	}

	// Move down at bottom — stays.
	um = mustUpdate(t, um, tea.KeyMsg{Type: tea.KeyDown})
	if um.Cursor() != 2 {
		t.Errorf("at bottom: cursor=%d, want 2", um.Cursor())
	}
}

func TestModel_VimKeys(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{
		sampleRecord("a", "b", ""),
		sampleRecord("c", "d", ""),
	})

	um := mustUpdate(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if um.Cursor() != 0 {
		t.Errorf("k: cursor=%d, want 0", um.Cursor())
	}

	um = mustUpdate(t, um, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if um.Cursor() != 1 {
		t.Errorf("j: cursor=%d, want 1", um.Cursor())
	}
}

func TestModel_EnterTree(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{sampleRecord("gw", "users", "trace-123")})
	um := mustUpdate(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if um.CurrentView() != tui.ViewTree {
		t.Errorf("view=%d, want viewTree", um.CurrentView())
	}
	if um.TreeOutput() == "" {
		t.Error("treeOutput should not be empty")
	}
}

func TestModel_EnterTree_NoTraceID(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{sampleRecord("gw", "users", "")})
	um := mustUpdate(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if um.CurrentView() != tui.ViewWatch {
		t.Errorf("view=%d, want viewWatch (no trace-id)", um.CurrentView())
	}
}

func TestModel_MapView(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{sampleRecord("gw", "users", "")})
	um := mustUpdate(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})

	if um.CurrentView() != tui.ViewMap {
		t.Errorf("view=%d, want viewMap", um.CurrentView())
	}
	if um.MapOutput() == "" {
		t.Error("mapOutput should not be empty")
	}
}

func TestModel_EscBackToWatch(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{sampleRecord("gw", "users", "trace-1")})
	// Enter tree view first.
	um := mustUpdate(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	um = mustUpdate(t, um, tea.KeyMsg{Type: tea.KeyEscape})

	if um.CurrentView() != tui.ViewWatch {
		t.Errorf("view=%d, want viewWatch", um.CurrentView())
	}
}

func TestModel_EOFMsg(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel(nil)
	updated, cmd := m.Update(tui.EOFMsg{})
	um := tui.AsModel(updated)

	if !um.Eof() {
		t.Error("eof should be true")
	}
	if cmd != nil {
		t.Error("cmd should be nil after EOF")
	}
}

func TestModel_ViewWatch_Empty(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel(nil)
	v := m.View()

	if !strings.Contains(v, "Waiting for traffic") {
		t.Errorf("empty view should show waiting message, got: %q", v)
	}
}

func TestModel_ViewWatch_WithRecords(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{sampleRecord("gw", "users", "")})
	v := m.View()

	if !strings.Contains(v, "▶") {
		t.Error("view should contain cursor marker")
	}
	if !strings.Contains(v, "gw") {
		t.Error("view should contain source name")
	}
	if !strings.Contains(v, "export") {
		t.Error("footer should mention export")
	}
}

func TestModel_ViewWatch_EOF(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel([]model.Record{sampleRecord("gw", "users", "")})
	um := mustUpdate(t, m, tui.EOFMsg{})
	v := um.View()

	if !strings.Contains(v, "EOF") {
		t.Error("footer should show EOF")
	}
}

func TestModel_StatusMsg_ClearedOnKey(t *testing.T) {
	t.Parallel()

	// Set status via ErrMsg, then clear via key press.
	m := tui.NewTestModel([]model.Record{sampleRecord("gw", "users", "")})
	m = mustUpdate(t, m, tui.ErrMsg{Err: errors.New("test")})

	um := mustUpdate(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if um.StatusMsg() != "" {
		t.Errorf("statusMsg should be cleared, got %q", um.StatusMsg())
	}
}

func TestModel_StatusMsg_ClearedOnRecordMsg(t *testing.T) {
	t.Parallel()

	m := tui.NewTestModel(nil)
	m = mustUpdate(t, m, tui.ErrMsg{Err: errors.New("test")})
	if m.StatusMsg() == "" {
		t.Fatal("statusMsg should be set after ErrMsg")
	}

	um := mustUpdate(t, m, tui.RecordMsg{Record: sampleRecord("a", "b", "")})
	if um.StatusMsg() != "" {
		t.Errorf("statusMsg should be cleared on RecordMsg, got %q", um.StatusMsg())
	}
}
