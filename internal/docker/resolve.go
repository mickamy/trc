package docker

import (
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

	svcMap := make(ServiceMap, len(containers))
	for _, c := range containers {
		ip := stripCIDR(c.IPv4Address)
		if ip == "" {
			continue
		}
		svcMap[ip] = cleanServiceName(c.Name)
	}
	return svcMap, nil
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
