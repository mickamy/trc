# trc

[![Sponsor](https://img.shields.io/badge/Sponsor-❤-ea4aaa?style=flat-square&logo=github)](https://github.com/sponsors/mickamy)

**Real-time HTTP/gRPC traffic tracer for Docker microservices**

![demo](docs/demo.gif)

## Features

- **HTTP & gRPC** — Captures both HTTP/1.1 and gRPC (HTTP/2) traffic on Docker networks
- **Three views** — Watch (live stream), Tree (trace correlation), Map (service dependency graph)
- **Trace correlation** — Groups requests into trace trees using `X-Request-Id` / `traceparent` headers
- **Filtering** — Filter output by service name with `--filter`
- **Export** — Export captured records as NDJSON for offline analysis
- **Offline replay** — Load a previously exported NDJSON file with `--input`

## Quick Start

```bash
# 1. Install
brew install mickamy/tap/trc

# 2. Start your Docker Compose services
cd example && docker compose up -d

# 3. Run trc
trc -n example_app
```

## Installation

### Homebrew

```bash
brew install mickamy/tap/trc
```

### go install

```bash
go install github.com/mickamy/trc@latest
```

### Build from source

```bash
git clone https://github.com/mickamy/trc.git
cd trc
make build      # outputs to bin/trc and bin/trcd
make install    # installs to $GOBIN
```

## Usage

### CLI Flags

| Flag              | Description                                          |
|-------------------|------------------------------------------------------|
| `-n`, `--network` | Docker network to capture (auto-detected if omitted) |
| `--filter`        | Filter output by service name                        |
| `-i`, `--input`   | Read NDJSON file into TUI (skip live capture)        |
| `-v`, `--version` | Print version                                        |
| `-h`, `--help`    | Show help                                            |

### TUI Key Bindings

#### Watch View

| Key              | Action                              |
|------------------|-------------------------------------|
| `↑`/`↓`, `j`/`k` | Move cursor                         |
| `Enter`          | Show trace tree for selected record |
| `m`              | Show service dependency map         |
| `e`              | Export all records as NDJSON        |
| `q`              | Quit                                |

#### Tree / Map View

| Key   | Action             |
|-------|--------------------|
| `Esc` | Back to watch view |
| `q`   | Quit               |

### Views

- **Watch** — Live scrolling list of captured HTTP/gRPC records. Auto-scrolls to the latest record.
- **Tree** — Visualizes all requests sharing the same trace ID as a call tree.
- **Map** — Shows a service dependency graph derived from captured traffic.

## How It Works

```
┌────────────────────────────────────────────┐
│  Docker Network                            │
│                                            │
│  ┌─────────┐  HTTP   ┌─────────┐           │
│  │ gateway │ ──────▶ │  users  │           │
│  └────┬────┘         └─────────┘           │
│       │  gRPC        ┌─────────┐           │
│       └────────────▶ │  auth   │           │
|                      └─────────┘           │
│                                            │
│  ┌──────────────────────────────────┐      │
│  │  trcd (sidecar container)        │      │
│  │  captures packets via gopacket   │      │
│  │  streams NDJSON to stdout        │      │
│  └────────────────┬─────────────────┘      │
└───────────────────┼────────────────────────┘
                    │ NDJSON
                    ▼
             ┌─────────────┐
             │  trc (CLI)  │
             │  Bubble Tea │
             │  TUI        │
             └─────────────┘
```

1. `trc` launches **trcd** as a Docker container attached to the target network.
2. **trcd** captures packets using `gopacket`, reassembles HTTP/gRPC exchanges, and streams them as NDJSON to stdout.
3. `trc` reads the NDJSON stream and renders it in an interactive TUI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Configuration

trc reads configuration from `.trc.yaml` in the current directory (project-local) and `~/.config/trc.yaml` (global). Project-local settings take priority.

```yaml
# .trc.yaml
command:
  runtime: docker          # container runtime (default: "docker")
daemon_image: ghcr.io/mickamy/trcd:latest  # trcd image (default)
```

## Example App

The `example/` directory contains a sample microservice application with five services:

| Service     | Role                              |
|-------------|-----------------------------------|
| **gateway** | HTTP entrypoint (port 8080)       |
| **users**   | User service (HTTP)               |
| **orders**  | Order service (HTTP, calls users) |
| **auth**    | Auth service (gRPC)               |
| **client**  | Automated request generator       |

```bash
cd example
docker compose up -d
trc -n example_app
```

## License

[MIT](LICENSE) &copy; 2026 Tetsuro Mikami
