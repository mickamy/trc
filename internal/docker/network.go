package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mickamy/trc/internal/exec"
)

// builtinNetworks are Docker's default networks that should be excluded
// from auto-detection.
var builtinNetworks = map[string]struct{}{
	"bridge": {},
	"host":   {},
	"none":   {},
}

// networkEntry represents a network from docker network ls.
type networkEntry struct {
	Name string `json:"Name"` //nolint:tagliatelle // Docker API uses PascalCase
}

// DetectNetwork validates the given network name, or auto-detects a custom
// Docker network if none is specified.
func DetectNetwork(ctx context.Context, runner exec.Runner, network string) (string, error) {
	if network != "" {
		return validateNetwork(ctx, runner, network)
	}
	return autoDetect(ctx, runner)
}

func validateNetwork(ctx context.Context, runner exec.Runner, network string) (string, error) {
	out, err := runner.RuntimeOutput(ctx,
		"network", "inspect", network, "--format", "{{.Name}}",
	)
	if err != nil {
		return "", fmt.Errorf("network %q not found: %w", network, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func autoDetect(ctx context.Context, runner exec.Runner) (string, error) {
	out, err := runner.RuntimeOutput(ctx, "network", "ls", "--format", "{{json .}}")
	if err != nil {
		return "", fmt.Errorf("listing networks: %w", err)
	}

	var candidates []string
	for line := range bytes.SplitSeq(out, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry networkEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if _, builtin := builtinNetworks[entry.Name]; builtin {
			continue
		}
		candidates = append(candidates, entry.Name)
	}

	switch len(candidates) {
	case 0:
		return "", errors.New("no custom Docker networks found; specify --network")
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf(
			"multiple custom networks found (%s); specify --network",
			strings.Join(candidates, ", "),
		)
	}
}
