package exec_test

import (
	"io"
	"testing"

	"github.com/mickamy/trc/internal/config"
	"github.com/mickamy/trc/internal/exec"
)

func TestRuntimeOutput(t *testing.T) {
	t.Parallel()

	r := exec.New(config.Config{
		Command: config.Command{Runtime: "echo"},
	})

	out, err := r.RuntimeOutput(t.Context(), "hello", "world")
	if err != nil {
		t.Fatalf("RuntimeOutput() error = %v", err)
	}

	got := string(out)
	want := "hello world\n"
	if got != want {
		t.Errorf("RuntimeOutput() = %q, want %q", got, want)
	}
}

func TestRuntimePipe(t *testing.T) {
	t.Parallel()

	r := exec.New(config.Config{
		Command: config.Command{Runtime: "echo"},
	})

	reader, cleanup, err := r.RuntimePipe(t.Context(), "piped", "output")
	if err != nil {
		t.Fatalf("RuntimePipe() error = %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}

	got := string(data)
	want := "piped output\n"
	if got != want {
		t.Errorf("RuntimePipe() = %q, want %q", got, want)
	}
}

func TestRuntimeOutput_EmptyCommand(t *testing.T) {
	t.Parallel()

	r := exec.New(config.Config{
		Command: config.Command{Runtime: ""},
	})

	_, err := r.RuntimeOutput(t.Context(), "arg")
	if err == nil {
		t.Fatal("RuntimeOutput() expected error for empty command, got nil")
	}
}

func TestRuntimePipe_EmptyCommand(t *testing.T) {
	t.Parallel()

	r := exec.New(config.Config{
		Command: config.Command{Runtime: ""},
	})

	_, _, err := r.RuntimePipe(t.Context(), "arg")
	if err == nil {
		t.Fatal("RuntimePipe() expected error for empty command, got nil")
	}
}
