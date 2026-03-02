package docker

import (
	"context"
	"fmt"

	"github.com/mickamy/trc/internal/exec"
)

// EnsureImage checks if the image exists locally and pulls it if not.
func EnsureImage(ctx context.Context, runner exec.Runner, image string) error {
	_, inspectErr := runner.RuntimeOutput(ctx, "image", "inspect", image)
	if inspectErr == nil {
		return nil
	}

	// Image not found (or inspect failed for another reason). Attempt to pull;
	// if the pull also fails, report both errors so the user can distinguish
	// "image missing" from operational failures (e.g. runtime unavailable).
	if pullErr := runner.Runtime(ctx, "pull", image); pullErr != nil {
		return fmt.Errorf("pulling image %s: %w (inspect had failed: %w)", image, pullErr, inspectErr)
	}
	return nil
}
