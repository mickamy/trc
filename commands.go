package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/mickamy/trc/internal/display/tui"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/exec"
)

func handleRun(ctx context.Context, flags globalFlags) error {
	if flags.input != "" {
		return handleInputFile(ctx, flags)
	}
	return handleLiveCapture(ctx, flags)
}

// handleInputFile opens a NDJSON file and launches the TUI with its contents.
func handleInputFile(ctx context.Context, flags globalFlags) error {
	f, err := os.Open(flags.input)
	if err != nil {
		return fmt.Errorf("opening input file: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Optionally resolve service names if --network is provided.
	var svcMap docker.ServiceMap
	if flags.network != "" {
		e, err := configure(flags)
		if err != nil {
			return err
		}
		svcMap, err = docker.ResolveServices(ctx, e.runner, flags.network)
		if err != nil {
			return fmt.Errorf("resolving services: %w", err)
		}
	}

	resolve := func() docker.ServiceMap { return svcMap }
	if err := tui.Run(ctx, f, resolve, flags.filter); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// handleLiveCapture starts a packet capture and launches the TUI.
func handleLiveCapture(ctx context.Context, flags globalFlags) error {
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
		// Use WithoutCancel so that container cleanup succeeds even
		// after the parent context has been cancelled (e.g. Ctrl-C).
		docker.StopCapture(context.WithoutCancel(ctx), e.runner)
		_ = cleanup()
	}()

	svcMap, err := docker.ResolveServices(ctx, e.runner, network)
	if err != nil {
		return fmt.Errorf("resolving services: %w", err)
	}

	// Periodically refresh the service map in the background.
	// Use a child context so the goroutine stops when the TUI returns.
	var mu sync.RWMutex
	refreshCtx, refreshCancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() {
		refreshServices(refreshCtx, e.runner, network, &mu, &svcMap)
	})

	resolve := func() docker.ServiceMap { return snapMap(&mu, &svcMap) }
	tuiErr := tui.Run(ctx, stdout, resolve, flags.filter)
	refreshCancel()
	wg.Wait()
	if tuiErr != nil {
		return fmt.Errorf("tui: %w", tuiErr)
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
	copied := make(docker.ServiceMap, len(*svcMap))
	maps.Copy(copied, *svcMap)
	return copied
}
