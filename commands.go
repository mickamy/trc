package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/mickamy/trc/internal/display"
	"github.com/mickamy/trc/internal/docker"
	"github.com/mickamy/trc/internal/exec"
	"github.com/mickamy/trc/internal/model"
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

func handleTree(ctx context.Context, flags globalFlags, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: trc tree <trace-id>")
	}
	traceID := args[0]

	records, svcMap, err := captureForDuration(ctx, flags)
	if err != nil {
		return err
	}

	display.PrintTree(os.Stdout, records, svcMap, traceID)
	return nil
}

func handleMap(ctx context.Context, flags globalFlags, _ []string) error {
	records, svcMap, err := captureForDuration(ctx, flags)
	if err != nil {
		return err
	}

	display.PrintMap(os.Stdout, records, svcMap)
	return nil
}

// captureForDuration starts a capture, collects records for flags.duration,
// and returns the collected records along with the service map.
func captureForDuration(
	ctx context.Context, flags globalFlags,
) ([]model.Record, docker.ServiceMap, error) {
	e, err := configure(flags)
	if err != nil {
		return nil, nil, err
	}

	network, err := docker.DetectNetwork(ctx, e.runner, flags.network)
	if err != nil {
		return nil, nil, fmt.Errorf("detecting network: %w", err)
	}

	if err := docker.EnsureImage(ctx, e.runner, e.cfg.DaemonImage); err != nil {
		return nil, nil, fmt.Errorf("ensuring image: %w", err)
	}

	stdout, cleanup, err := docker.StartCapture(ctx, e.runner, e.cfg.DaemonImage, network)
	if err != nil {
		return nil, nil, fmt.Errorf("starting capture: %w", err)
	}
	defer func() {
		docker.StopCapture(ctx, e.runner)
		_ = cleanup()
	}()

	svcMap, err := docker.ResolveServices(ctx, e.runner, network)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving services: %w", err)
	}

	fmt.Fprintf(os.Stderr, "capturing for %s...\n", flags.duration)

	records, err := collectRecords(ctx, stdout, flags.duration)
	if err != nil {
		return nil, nil, err
	}

	return records, svcMap, nil
}

// collectRecords reads NDJSON records from r for the given duration.
func collectRecords(
	ctx context.Context, r io.Reader, d time.Duration,
) ([]model.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()

	var records []model.Record
	scanner := bufio.NewScanner(r)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for scanner.Scan() {
			var rec model.Record
			if err := rec.UnmarshalNDJSON(scanner.Bytes()); err != nil {
				continue
			}
			records = append(records, rec)
		}
	}()

	select {
	case <-ctx.Done():
	case <-done:
	}

	return records, nil
}
