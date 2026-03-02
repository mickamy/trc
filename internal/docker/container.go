package docker

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mickamy/trc/internal/exec"
)

const containerName = "trcd"

// StartCapture starts the trcd container with host networking and returns
// a reader for its stdout (NDJSON stream). The trcd process captures traffic
// on the bridge interface (br-<id>) corresponding to the Docker network.
// The caller must call the returned cleanup function when done.
func StartCapture(
	ctx context.Context, runner exec.Runner, image, network string,
	args ...string,
) (io.ReadCloser, func() error, error) {
	// Remove any leftover container from a previous run.
	_, _ = runner.RuntimeOutput(ctx, "rm", "-f", containerName)

	iface, err := bridgeInterface(ctx, runner, network)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving bridge interface: %w", err)
	}

	cmdArgs := buildRunArgs(image, iface)
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

// bridgeInterface resolves the host bridge interface name (br-<short-id>)
// for the given Docker network.
func bridgeInterface(
	ctx context.Context, runner exec.Runner, network string,
) (string, error) {
	out, err := runner.RuntimeOutput(ctx,
		"network", "inspect", network, "--format", "{{.Id}}",
	)
	if err != nil {
		return "", fmt.Errorf("inspecting network %q: %w", network, err)
	}
	id := strings.TrimSpace(string(out))
	if len(id) < 12 { //nolint:mnd // Docker short ID length
		return "", fmt.Errorf("unexpected network id %q", id)
	}
	return "br-" + id[:12], nil
}

func buildRunArgs(image, iface string) []string {
	return []string{
		"run", "--rm", "-i",
		"--name", containerName,
		"--net=host",
		"--cap-add", "NET_RAW",
		"--cap-add", "NET_ADMIN",
		image,
		"-i", iface,
	}
}
