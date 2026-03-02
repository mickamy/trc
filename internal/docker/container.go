package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/mickamy/trc/internal/exec"
)

const containerName = "trcd"

// StartCapture starts the trcd container on the given network and returns
// a reader for its stdout (NDJSON stream). The caller must call the returned
// cleanup function when done. Extra args are passed to the trcd command.
func StartCapture(
	ctx context.Context, runner exec.Runner, image, network string,
	args ...string,
) (io.ReadCloser, func() error, error) {
	// Remove any leftover container from a previous run.
	_, _ = runner.RuntimeOutput(ctx, "rm", "-f", containerName)

	cmdArgs := buildRunArgs(image, network)
	cmdArgs = append(cmdArgs, args...)

	r, cleanup, err := runner.RuntimePipe(ctx, cmdArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("starting capture container: %w", err)
	}
	return r, cleanup, nil
}

// StopCapture idempotently stops and removes the trcd container.
func StopCapture(ctx context.Context, runner exec.Runner) {
	cleanupCtx := context.WithoutCancel(ctx)
	_, _ = runner.RuntimeOutput(cleanupCtx, "rm", "-f", containerName)
}

func buildRunArgs(image, network string) []string {
	return []string{
		"run", "--rm", "-i",
		"--name", containerName,
		"--network", network,
		"--cap-add", "NET_RAW",
		"--cap-add", "NET_ADMIN",
		image,
	}
}
