package docker

import (
	"context"
	"fmt"

	"github.com/mickamy/trc/internal/exec"
)

// EnsureImage checks if the image exists locally and pulls it if not.
func EnsureImage(ctx context.Context, runner exec.Runner, image string) error {
	_, err := runner.RuntimeOutput(ctx, "image", "inspect", image)
	if err == nil {
		return nil
	}

	if err := runner.Runtime(ctx, "pull", image); err != nil {
		return fmt.Errorf("pulling image %s: %w", image, err)
	}
	return nil
}
