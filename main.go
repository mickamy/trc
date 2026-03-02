package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	var (
		flags       globalFlags
		showVersion bool
	)

	fs := flag.NewFlagSet("trc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = printUsage
	fs.StringVar(&flags.network, "network", "", "")
	fs.StringVar(&flags.network, "n", "", "")
	fs.StringVar(&flags.filter, "filter", "", "")
	fs.StringVar(&flags.input, "input", "", "")
	fs.StringVar(&flags.input, "i", "", "")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&showVersion, "v", false, "")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if showVersion {
		fmt.Println("trc", version)
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := handleRun(ctx, flags); err != nil {
		fmt.Fprintf(os.Stderr, "trc: %v\n", err)
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `trc - Docker network HTTP/gRPC traffic tracer

Usage:
  trc [flags]

Flags:
  -n, --network    Docker network to capture (auto-detected if omitted)
  --filter         Filter output by service name
  -i, --input      Read NDJSON file into TUI (skip live capture)
  --version, -v    Print version
  -h, --help       Show this help

Keys (TUI):
  ↑/↓, j/k         Move cursor
  Enter             Show trace tree for selected record
  m                 Show service dependency map
  e                 Export all records as NDJSON
  Esc               Back to watch view
  q                 Quit
`)
}
