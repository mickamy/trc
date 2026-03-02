package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickamy/trc/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(t.TempDir(), filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Command.Runtime != config.DefaultRuntimeCommand {
		t.Errorf("Runtime = %q, want %q", cfg.Command.Runtime, config.DefaultRuntimeCommand)
	}
	if cfg.DaemonImage != config.DefaultDaemonImage {
		t.Errorf("DaemonImage = %q, want %q", cfg.DaemonImage, config.DefaultDaemonImage)
	}
}

func TestLoad_GlobalOnly(t *testing.T) {
	t.Parallel()

	globalDir := t.TempDir()
	globalPath := filepath.Join(globalDir, "trc.yaml")
	writeFile(t, globalPath, `
command:
  runtime: podman
`)

	cfg, err := config.Load(t.TempDir(), globalPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Command.Runtime != "podman" {
		t.Errorf("Runtime = %q, want %q", cfg.Command.Runtime, "podman")
	}
	if cfg.DaemonImage != config.DefaultDaemonImage {
		t.Errorf("DaemonImage = %q, want %q", cfg.DaemonImage, config.DefaultDaemonImage)
	}
}

func TestLoad_LocalOverridesGlobal(t *testing.T) {
	t.Parallel()

	globalDir := t.TempDir()
	globalPath := filepath.Join(globalDir, "trc.yaml")
	writeFile(t, globalPath, `
command:
  runtime: podman
daemon_image: ghcr.io/other/trcd:v1
`)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".trc.yaml"), `
daemon_image: ghcr.io/local/trcd:dev
`)

	cfg, err := config.Load(projectDir, globalPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Command.Runtime != "podman" {
		t.Errorf("Runtime = %q, want %q (from global)", cfg.Command.Runtime, "podman")
	}
	if cfg.DaemonImage != "ghcr.io/local/trcd:dev" {
		t.Errorf("DaemonImage = %q, want %q (local overrides global)", cfg.DaemonImage, "ghcr.io/local/trcd:dev")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".trc.yaml"), `{{{invalid`)

	_, err := config.Load(dir, filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML, got nil")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}
