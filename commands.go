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

// teeReadCloser combines an io.Reader with an io.Closer so that a TeeReader
// can be closed (closing the underlying source) to unblock a blocking read.
type teeReadCloser struct {
	io.Reader
	io.Closer
}

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
	var mu sync.RWMutex
	var wg sync.WaitGroup
	wg.Go(func() {
		refreshServices(ctx, e.runner, network, &mu, &svcMap)
	})

	// Optionally tee raw NDJSON to a file for later analysis.
	var r io.Reader = stdout
	if flags.output != "" {
		f, err := os.Create(flags.output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer func() { _ = f.Close() }()
		r = teeReadCloser{Reader: io.TeeReader(stdout, f), Closer: stdout}
	}

	resolve := func() docker.ServiceMap { return snapMap(&mu, &svcMap) }
	watchErr := display.StreamWatch(ctx, r, resolve, flags.filter, os.Stdout)
	wg.Wait()
	if watchErr != nil {
		return fmt.Errorf("streaming watch: %w", watchErr)
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
	for k, v := range *svcMap {
		copied[k] = v
	}
	return copied
}

func handleTree(ctx context.Context, flags globalFlags, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: trc tree <trace-id>")
	}
	traceID := args[0]

	records, svcMap, err := loadOrCapture(ctx, flags)
	if err != nil {
		return err
	}

	display.PrintTree(os.Stdout, records, svcMap, traceID)
	return nil
}

func handleMap(ctx context.Context, flags globalFlags, _ []string) error {
	records, svcMap, err := loadOrCapture(ctx, flags)
	if err != nil {
		return err
	}

	display.PrintMap(os.Stdout, records, svcMap)
	return nil
}

// loadOrCapture reads records from a file (--input) or performs a live
// capture for the configured duration.
func loadOrCapture(
	ctx context.Context, flags globalFlags,
) ([]model.Record, docker.ServiceMap, error) {
	e, err := configure(flags)
	if err != nil {
		return nil, nil, err
	}

	if flags.input != "" {
		return loadFromFile(ctx, e, flags)
	}
	return captureForDuration(ctx, e, flags)
}

// loadFromFile reads NDJSON records from a file and optionally resolves
// service names if --network is provided.
func loadFromFile(
	ctx context.Context, e env, flags globalFlags,
) ([]model.Record, docker.ServiceMap, error) {
	f, err := os.Open(flags.input)
	if err != nil {
		return nil, nil, fmt.Errorf("opening input file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var records []model.Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) //nolint:mnd // 10 MiB max token size
	for scanner.Scan() {
		var rec model.Record
		if err := rec.UnmarshalNDJSON(scanner.Bytes()); err != nil {
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading input file: %w", err)
	}

	var svcMap docker.ServiceMap
	if flags.network != "" {
		svcMap, err = docker.ResolveServices(ctx, e.runner, flags.network)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving services: %w", err)
		}
	}

	return records, svcMap, nil
}

// captureForDuration starts a capture, collects records for flags.duration,
// and returns the collected records along with the service map.
func captureForDuration(
	ctx context.Context, e env, flags globalFlags,
) ([]model.Record, docker.ServiceMap, error) {
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
		// Use WithoutCancel so that container cleanup succeeds even
		// after the parent context has been cancelled (e.g. Ctrl-C).
		docker.StopCapture(context.WithoutCancel(ctx), e.runner)
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
// The scanner runs in a separate goroutine so that ctx cancellation is
// handled immediately even when Scan() is blocked on I/O.  The caller
// (captureForDuration) closes the reader via docker.StopCapture /
// cleanup(), which unblocks the scanner goroutine.
func collectRecords(
	ctx context.Context, r io.Reader, d time.Duration,
) ([]model.Record, error) {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()

	recCh := make(chan model.Record)
	errCh := make(chan error, 1)
	go func() {
		defer close(recCh)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) //nolint:mnd // 10 MiB max token size
		for scanner.Scan() {
			var rec model.Record
			if err := rec.UnmarshalNDJSON(scanner.Bytes()); err != nil {
				continue
			}
			select {
			case recCh <- rec:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	var records []model.Record
	for {
		select {
		case <-ctx.Done():
			return records, nil
		case rec, ok := <-recCh:
			if !ok {
				select {
				case err := <-errCh:
					return records, fmt.Errorf("reading records: %w", err)
				default:
					return records, nil
				}
			}
			records = append(records, rec)
		}
	}
}
