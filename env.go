package main

import (
	"fmt"

	"github.com/mickamy/trc/internal/config"
	"github.com/mickamy/trc/internal/exec"
)

// env holds the common dependencies resolved once per command.
type env struct {
	cfg    config.Config
	runner exec.Runner
}

func configure(_ globalFlags) (env, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return env{}, fmt.Errorf("loading config: %w", err)
	}

	return env{
		cfg:    cfg,
		runner: exec.New(cfg),
	}, nil
}
