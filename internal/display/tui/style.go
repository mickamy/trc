package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var borderColor = lipgloss.Color("240")

// Column content widths for the watch view (excluding inter-column separators).
const (
	colTime     = 12 // "15:04:05.000"
	colProto    = 5  // "http1" / "grpc"
	colSrc      = 12
	colDst      = 12
	colStatus   = 4 // "200" / "OK"
	colDuration = 8 // right-aligned, must fit "Duration" header label
)

// colFixedWatch is the total fixed-width portion of a watch row including
// marker and all separators.
//
//	marker(2) + time(12) + 1 + proto(5) + 1 + src(12) + " → "(3)
//	+ dst(12) + 1 + request(flex) + 1 + status(4) + 1 + dur(8) = 63
const colFixedWatch = 2 + colTime + 1 + colProto + 1 + colSrc + 3 + colDst + 1 + 1 + colStatus + 1 + colDuration

// renderBorderedBox creates a lipgloss bordered box with a custom title
// in the top border.
func renderBorderedBox(content, title string, innerWidth int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(innerWidth).
		BorderForeground(borderColor).
		Render(content)

	lines := strings.Split(box, "\n")
	if len(lines) > 0 {
		lines[0] = buildTopBorder(title, innerWidth)
	}

	return strings.Join(lines, "\n")
}

// renderBorderedBoxWithHelp creates a bordered box with a title in the top
// border and faint help text in the bottom border.
func renderBorderedBoxWithHelp(content, title, help string, innerWidth int) string {
	box := renderBorderedBox(content, title, innerWidth)

	lines := strings.Split(box, "\n")
	if n := len(lines); n > 0 {
		lines[n-1] = buildBottomBorder(help, innerWidth)
	}

	return strings.Join(lines, "\n")
}

func buildTopBorder(title string, innerWidth int) string {
	title = truncateStr(title, innerWidth)
	borderFg := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Bold(true)
	dashes := max(innerWidth-len([]rune(title)), 0)
	return borderFg.Render("╭") +
		titleStyle.Render(title) +
		borderFg.Render(strings.Repeat("─", dashes)+"╮")
}

func buildBottomBorder(help string, innerWidth int) string {
	help = truncateStr(help, innerWidth)
	borderFg := lipgloss.NewStyle().Foreground(borderColor)
	dashes := max(innerWidth-len([]rune(help)), 0)
	return borderFg.Render("╰") +
		lipgloss.NewStyle().Faint(true).Render(help) +
		borderFg.Render(strings.Repeat("─", dashes)+"╯")
}

// truncateStr truncates s to maxLen runes, appending "…" if truncated.
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

// formatDur formats a duration in milliseconds for display.
func formatDur(ms float64) string {
	if ms >= 1 {
		return fmt.Sprintf("%dms", int(ms))
	}
	us := ms * 1000 //nolint:mnd // ms to µs
	return fmt.Sprintf("%dµs", int(us))
}

// formatStat formats a record's status for the watch view.
func formatStat(proto string, status int, grpcStatus string) string {
	if proto == "grpc" && grpcStatus != "" {
		return grpcStatus
	}
	return strconv.Itoa(status)
}
