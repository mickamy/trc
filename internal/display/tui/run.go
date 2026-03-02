package tui

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mickamy/trc/internal/docker"
)

// Run starts the bubbletea TUI. It reads NDJSON records from r, resolves
// service names via svcMapFn, and optionally filters by service name.
// It blocks until the user quits or ctx is cancelled.
func Run(
	ctx context.Context, r io.Reader,
	svcMapFn func() docker.ServiceMap, filter string,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := startReader(ctx, r, svcMapFn, filterString(filter))
	m := newModel(ch, svcMapFn)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running tui: %w", err)
	}
	return nil
}
