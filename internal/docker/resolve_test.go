package docker_test

import (
	"testing"

	"github.com/mickamy/trc/internal/docker"
)

func TestCleanServiceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full compose name",
			input: "myproject-users-1",
			want:  "users",
		},
		{
			name:  "with leading slash",
			input: "/myproject-users-1",
			want:  "users",
		},
		{
			name:  "multi-digit replica",
			input: "app-orders-12",
			want:  "orders",
		},
		{
			name:  "no replica suffix",
			input: "myproject-gateway",
			want:  "gateway",
		},
		{
			name:  "simple name no prefix",
			input: "redis",
			want:  "redis",
		},
		{
			name:  "hyphenated service name",
			input: "myproject-auth-service-1",
			want:  "auth-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := docker.CleanServiceName(tt.input)
			if got != tt.want {
				t.Errorf("cleanServiceName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripCIDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"172.18.0.3/16", "172.18.0.3"},
		{"10.0.0.1/24", "10.0.0.1"},
		{"192.168.1.1", "192.168.1.1"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := docker.StripCIDR(tt.input)
			if got != tt.want {
				t.Errorf("stripCIDR(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
