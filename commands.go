package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mickamy/trc/internal/display"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/exec"
)

func handleWatch(ctx context.Context, flags globalFlags, _ []string) error {
	e, err := configure(flags)
	if err != nil {
		return err
	}

	network, err := docker.DetectNetwork(ctx, e.runner, flags.network)
	if err != nil {
		return fmt.Errorf("detecting network: %w", err)
	}

	if err := docker.EnsureImage(ctx, e.runner, e.cfg.DaemonImage); err != nil {
		return fmt.Errorf("ensuring image: %w", err)
	}

	stdout, cleanup, err := docker.StartCapture(ctx, e.runner, e.cfg.DaemonImage, network)
	if err != nil {
		return fmt.Errorf("starting capture: %w", err)
	}
	defer func() {
		docker.StopCapture(ctx, e.runner)
		_ = cleanup()
	}()

	svcMap, err := docker.ResolveServices(ctx, e.runner, network)
	if err != nil {
		return fmt.Errorf("resolving services: %w", err)
	}

	// Periodically refresh the service map in the background.
	var mu sync.RWMutex
	go refreshServices(ctx, e.runner, network, &mu, &svcMap)

	if err := display.StreamWatch(ctx, stdout, snapMap(&mu, &svcMap), flags.filter, os.Stdout); err != nil {
		return fmt.Errorf("streaming watch: %w", err)
	}
	return nil
}

func refreshServices(
	ctx context.Context, runner exec.Runner,
	network string, mu *sync.RWMutex, svcMap *docker.ServiceMap,
) {
	ticker := time.NewTicker(10 * time.Second) //nolint:mnd // refresh interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updated, err := docker.ResolveServices(ctx, runner, network)
			if err != nil {
				continue
			}
			mu.Lock()
			*svcMap = updated
			mu.Unlock()
		}
	}
}

func snapMap(mu *sync.RWMutex, svcMap *docker.ServiceMap) docker.ServiceMap {
	mu.RLock()
	defer mu.RUnlock()
	return *svcMap
}

func handleTree(_ context.Context, _ globalFlags, _ []string) error {
	return errors.New("not implemented: tree")
}

func handleMap(_ context.Context, _ globalFlags, _ []string) error {
	return errors.New("not implemented: map")
}
