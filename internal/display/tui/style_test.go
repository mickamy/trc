package tui_test

import (
	"strings"
	"testing"

	"github.com/mickamy/trc/internal/display/tui"
)

func TestRenderBorderedBox(t *testing.T) {
	t.Parallel()

	box := tui.RenderBorderedBox("hello world", " Title ", 40)

	if !strings.Contains(box, "Title") {
		t.Error("box should contain title text")
	}
	if !strings.Contains(box, "hello world") {
		t.Error("box should contain content text")
	}
	if !strings.Contains(box, "╭") {
		t.Error("box should have top-left corner")
	}
	if !strings.Contains(box, "╯") {
		t.Error("box should have bottom-right corner")
	}
}

func TestRenderBorderedBox_MultiLine(t *testing.T) {
	t.Parallel()

	content := "line1\nline2\nline3"
	box := tui.RenderBorderedBox(content, " Title ", 40)

	lines := strings.Split(box, "\n")
	// top border + 3 content lines + bottom border = 5
	if len(lines) < 5 {
		t.Errorf("expected at least 5 lines, got %d", len(lines))
	}
}

func TestRenderBorderedBoxWithHelp(t *testing.T) {
	t.Parallel()

	box := tui.RenderBorderedBoxWithHelp("content", " Title ", " q: quit ", 40)

	if !strings.Contains(box, "Title") {
		t.Error("box should contain title")
	}
	if !strings.Contains(box, "content") {
		t.Error("box should contain content")
	}
	if !strings.Contains(box, "q: quit") {
		t.Error("box should contain help text in bottom border")
	}
	if !strings.Contains(box, "╰") {
		t.Error("box should have bottom-left corner")
	}
}

func TestTruncateStr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"no truncation", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello w…"},
		{"maxLen 1", "hello", 1, "h"},
		{"empty", "", 5, ""},
		{"unicode", "あいうえお", 3, "あい…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tui.TruncateStr(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFormatDur(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ms   float64
		want string
	}{
		{"milliseconds", 42, "42ms"},
		{"sub-millisecond", 0.5, "500µs"},
		{"exactly 1ms", 1, "1ms"},
		{"large", 1234, "1234ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tui.FormatDur(tt.ms)
			if got != tt.want {
				t.Errorf("formatDur(%v) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}

func TestFormatStat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		proto      string
		status     int
		grpcStatus string
		want       string
	}{
		{"HTTP 200", "http1", 200, "", "200"},
		{"HTTP 404", "http1", 404, "", "404"},
		{"gRPC OK", "grpc", 0, "OK", "OK"},
		{"gRPC empty status", "grpc", 0, "", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tui.FormatStat(tt.proto, tt.status, tt.grpcStatus)
			if got != tt.want {
				t.Errorf("formatStat(%q, %d, %q) = %q, want %q",
					tt.proto, tt.status, tt.grpcStatus, got, tt.want)
			}
		})
	}
}
