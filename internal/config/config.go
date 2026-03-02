package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultRuntimeCommand = "docker"
	DefaultDaemonImage    = "ghcr.io/mickamy/trcd:latest"
)

// Command configures which container runtime binary to invoke.
type Command struct {
	Runtime string `yaml:"runtime"`
}

// Config holds all trc settings, resolved from global + local files.
type Config struct {
	Command     Command `yaml:"command"`
	DaemonImage string  `yaml:"daemon_image"`
}

func defaults() Config {
	return Config{
		Command: Command{
			Runtime: DefaultRuntimeCommand,
		},
		DaemonImage: DefaultDaemonImage,
	}
}

// Load reads config from .trc.yaml in projectDir (project-local) and
// globalPath (global), merging with project-local taking priority.
// Both files are optional; missing files are silently ignored.
func Load(projectDir, globalPath string) (Config, error) {
	cfg := defaults()

	if globalPath != "" {
		global, err := loadFile(globalPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return cfg, err
		}
		if err == nil {
			merge(&cfg, global)
		}
	}

	local, err := loadFile(filepath.Join(projectDir, ".trc.yaml"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cfg, err
	}
	if err == nil {
		merge(&cfg, local)
	}

	return cfg, nil
}

// LoadDefault loads config using the current directory and the standard
// global config path (~/.config/trc.yaml).
func LoadDefault() (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return defaults(), err
	}

	var globalPath string
	if home, err := os.UserHomeDir(); err == nil {
		globalPath = filepath.Join(home, ".config", "trc.yaml")
	}

	return Load(cwd, globalPath)
}

func loadFile(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path) //nolint:gosec // path is from known config locations
	if err != nil {
		return cfg, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}

func merge(base *Config, override Config) {
	if override.Command.Runtime != "" {
		base.Command.Runtime = override.Command.Runtime
	}
	if override.DaemonImage != "" {
		base.DaemonImage = override.DaemonImage
	}
}
