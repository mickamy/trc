package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/mickamy/trc/internal/capture"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	cfg := capture.DefaultConfig()

	var showVersion bool

	fs := flag.NewFlagSet("trcd", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&cfg.Interface, "interface", cfg.Interface, "network interface to capture")
	fs.StringVar(&cfg.Interface, "i", cfg.Interface, "network interface to capture (short)")
	fs.StringVar(&cfg.BPFFilter, "bpf", cfg.BPFFilter, "BPF filter expression")
	fs.BoolVar(&showVersion, "version", false, "")
	fs.BoolVar(&showVersion, "v", false, "")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if showVersion {
		fmt.Println("trcd", version)
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := capture.Run(ctx, cfg, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "trcd: %v\n", err)
		return 1
	}
	return 0
}
