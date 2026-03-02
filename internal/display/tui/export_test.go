package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/model"
)

// NewTestModel creates a Model for testing with the given records pre-loaded.
func NewTestModel(records []model.Record) Model {
	ch := make(chan model.Record)
	close(ch)
	m := newModel(ch, func() docker.ServiceMap { return docker.ServiceMap{} })
	m.records = records
	if len(records) > 0 {
		m.cursor = len(records) - 1
	}
	m.width = 120
	m.height = 24 //nolint:mnd // test default
	return m
}

// StartReader is an exported alias for startReader, for use in tests.
var StartReader = startReader

// WaitForRecord is an exported alias for waitForRecord, for use in tests.
var WaitForRecord = waitForRecord

// Expose view constants for tests.
const (
	ViewWatch = viewWatch
	ViewTree  = viewTree
	ViewMap   = viewMap
)

// CurrentView returns the current view of the model.
func (m Model) CurrentView() view { return m.currentView }

// Records returns the model's records slice.
func (m Model) Records() []model.Record { return m.records }

// Cursor returns the current cursor position.
func (m Model) Cursor() int { return m.cursor }

// AutoScroll returns whether auto-scroll is enabled.
func (m Model) AutoScroll() bool { return m.autoScroll }

// Eof returns whether the model has reached EOF.
func (m Model) Eof() bool { return m.eof }

// StatusMsg returns the current status message.
func (m Model) StatusMsg() string { return m.statusMsg }

// TreeOutput returns the tree view output.
func (m Model) TreeOutput() string { return m.treeOutput }

// MapOutput returns the map view output.
func (m Model) MapOutput() string { return m.mapOutput }

// AsModel converts a tea.Model back to a Model for testing.
func AsModel(t tea.Model) Model {
	return t.(Model) //nolint:forcetypeassert // test helper, panic is fine
}
