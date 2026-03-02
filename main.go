package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"
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
	fs.DurationVar(&flags.duration, "duration", 30*time.Second, "") //nolint:mnd // default capture window
	fs.DurationVar(&flags.duration, "d", 30*time.Second, "")        //nolint:mnd // short alias
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

	rest := fs.Args()
	if len(rest) == 0 {
		printUsage()
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	switch rest[0] {
	case "watch":
		err = handleWatch(ctx, flags, rest[1:])
	case "tree":
		err = handleTree(ctx, flags, rest[1:])
	case "map":
		err = handleMap(ctx, flags, rest[1:])
	default:
		fmt.Fprintf(os.Stderr, "trc: unknown command %q\n", rest[0])
		printUsage()
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "trc: %v\n", err)
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `trc - Docker network HTTP/gRPC traffic tracer

Usage:
  trc [flags] <command> [args...]

Commands:
  watch    Stream live traffic on a Docker network
  tree     Display a request chain as a tree (by trace-id)
  map      Show service dependency summary

Flags:
  -n, --network    Docker network to capture (auto-detected if omitted)
  --filter         Filter output by service name
  -d, --duration   Capture duration for tree/map (default 30s)
  --version, -v    Print version
  -h, --help       Show this help
`)
}
