package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mickamy/trc/internal/config"
)

var errEmptyCommand = errors.New("empty command")

// Runner abstracts container runtime command execution.
type Runner interface {
	Runtime(ctx context.Context, args ...string) error
	RuntimeOutput(ctx context.Context, args ...string) ([]byte, error)
	// RuntimePipe starts a runtime command and returns its stdout as a reader.
	// The caller must call the returned cleanup function when done.
	RuntimePipe(ctx context.Context, args ...string) (io.ReadCloser, func() error, error)
}

type runner struct {
	cfg config.Config
}

// New creates a Runner from the given config.
func New(cfg config.Config) Runner {
	return &runner{cfg: cfg}
}

// Runtime runs a runtime command (e.g. "docker run") with the given args,
// inheriting stdin/stdout/stderr.
func (r *runner) Runtime(ctx context.Context, args ...string) error {
	bin, baseArgs, err := buildArgs(r.cfg.Command.Runtime)
	if err != nil {
		return err
	}
	cmd := buildCmd(ctx, bin, baseArgs, args)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %s: %w", bin, err)
	}
	return nil
}

// RuntimeOutput runs a runtime command and returns its combined output.
func (r *runner) RuntimeOutput(ctx context.Context, args ...string) ([]byte, error) {
	bin, baseArgs, err := buildArgs(r.cfg.Command.Runtime)
	if err != nil {
		return nil, err
	}
	cmd := buildCmd(ctx, bin, baseArgs, args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("exec %s: %w", bin, err)
	}
	return out, nil
}

// RuntimePipe starts a runtime command and pipes its stdout back to the caller.
// Stderr is inherited. The returned cleanup function waits for the process to
// exit and must be called after the reader is fully consumed or no longer needed.
func (r *runner) RuntimePipe(ctx context.Context, args ...string) (io.ReadCloser, func() error, error) {
	bin, baseArgs, err := buildArgs(r.cfg.Command.Runtime)
	if err != nil {
		return nil, nil, err
	}
	cmd := buildCmd(ctx, bin, baseArgs, args)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start %s: %w", bin, err)
	}

	cleanup := func() error {
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("wait %s: %w", bin, err)
		}
		return nil
	}
	return stdout, cleanup, nil
}

func buildArgs(command string) (string, []string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil, errEmptyCommand
	}
	return parts[0], parts[1:], nil
}

func buildCmd(ctx context.Context, bin string, baseArgs, args []string) *exec.Cmd {
	cmdArgs := make([]string, 0, len(baseArgs)+len(args))
	cmdArgs = append(cmdArgs, baseArgs...)
	cmdArgs = append(cmdArgs, args...)
	return exec.CommandContext(ctx, bin, cmdArgs...) //nolint:gosec // command comes from user config
}
