package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickamy/trc/internal/exec"
)

// ServiceMap maps container IP addresses to human-readable service names.
type ServiceMap map[string]string

// containerInfo represents a container entry from docker network inspect.
type containerInfo struct {
	Name        string `json:"Name"`        //nolint:tagliatelle // Docker API uses PascalCase
	IPv4Address string `json:"IPv4Address"` //nolint:tagliatelle // Docker API uses PascalCase
}

// ResolveServices queries the Docker network and builds a map from
// container IP to a cleaned-up service name.
func ResolveServices(ctx context.Context, runner exec.Runner, network string) (ServiceMap, error) {
	out, err := runner.RuntimeOutput(ctx,
		"network", "inspect", network, "--format", "{{json .Containers}}",
	)
	if err != nil {
		return nil, fmt.Errorf("inspecting network %s: %w", network, err)
	}

	var containers map[string]containerInfo
	if err := json.Unmarshal(out, &containers); err != nil {
		return nil, fmt.Errorf("parsing network containers: %w", err)
	}

	// Try to resolve compose service labels in a single docker inspect call.
	// Falls back to the name-based heuristic for non-compose containers.
	labels := resolveComposeLabels(ctx, runner, containers)

	svcMap := make(ServiceMap, len(containers))
	for id, c := range containers {
		ip := stripCIDR(c.IPv4Address)
		if ip == "" {
			continue
		}
		if label, ok := labels[id]; ok {
			svcMap[ip] = label
		} else {
			svcMap[ip] = cleanServiceName(c.Name)
		}
	}
	return svcMap, nil
}

// resolveComposeLabels runs a single docker inspect on all container IDs and
// returns a map from container ID to the com.docker.compose.service label.
// Returns nil on any error (caller falls back to name heuristic).
func resolveComposeLabels(
	ctx context.Context, runner exec.Runner, containers map[string]containerInfo,
) map[string]string {
	if len(containers) == 0 {
		return nil
	}

	ids := make([]string, 0, len(containers))
	for id := range containers {
		ids = append(ids, id)
	}

	args := make([]string, 0, len(ids)+3)
	args = append(args,
		"inspect",
		"--format", `{{.Id}}{{"\t"}}{{index .Config.Labels "com.docker.compose.service"}}`,
	)
	args = append(args, ids...)

	out, err := runner.RuntimeOutput(ctx, args...)
	if err != nil {
		return nil
	}

	labels := make(map[string]string, len(containers))
	for line := range bytes.SplitSeq(out, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		id, label, ok := strings.Cut(string(line), "\t")
		if !ok || label == "" || label == "<no value>" {
			continue
		}
		labels[id] = label
	}
	return labels
}

// stripCIDR removes the "/prefix" suffix from a CIDR notation address.
func stripCIDR(addr string) string {
	ip, _, found := strings.Cut(addr, "/")
	if found {
		return ip
	}
	return addr
}

// cleanServiceName removes the compose project prefix and replica suffix
// from a container name. e.g. "myproject-users-1" -> "users".
func cleanServiceName(name string) string {
	// Remove leading slash if present (docker inspect sometimes includes it).
	name = strings.TrimPrefix(name, "/")

	// Remove replica suffix: "-1", "-2", etc.
	if idx := strings.LastIndex(name, "-"); idx >= 0 {
		suffix := name[idx+1:]
		if isDigits(suffix) {
			name = name[:idx]
		}
	}

	// Remove project prefix: "project-service" -> "service".
	// The prefix is everything before the first hyphen.
	if idx := strings.IndexByte(name, '-'); idx >= 0 {
		name = name[idx+1:]
	}

	return name
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
