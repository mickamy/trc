BUILD_DIR = bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: all build build-trc build-trcd install uninstall docker clean test lint

all: build

build: build-trc build-trcd

build-trc:
	@echo "Building..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/trc .

build-trcd:
	@echo "Building trcd..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/trcd ./cmd/trcd/

install:
	@echo "Installing trc and trcd..."
	@bin_dir=$$(go env GOBIN); \
	if [ -z "$$bin_dir" ]; then \
		bin_dir=$$(go env GOPATH)/bin; \
	fi; \
	mkdir -p "$$bin_dir"; \
	echo "Installing to $$bin_dir"; \
	go build $(LDFLAGS) -o "$$bin_dir/trc" . && \
	go build $(LDFLAGS) -o "$$bin_dir/trcd" ./cmd/trcd/

uninstall:
	@echo "Uninstalling trc and trcd..."
	@bin_dir=$$(go env GOBIN); \
	if [ -z "$$bin_dir" ]; then \
		bin_dir=$$(go env GOPATH)/bin; \
	fi; \
	rm -f "$$bin_dir/trc" "$$bin_dir/trcd"

docker:
	@echo "Building trcd Docker image..."
	docker build -t ghcr.io/mickamy/trcd:latest .

clean:
	@echo "Cleaning up..."
	rm -rf $(BUILD_DIR)

test:
	go test -race ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed"; \
		exit 1; \
	}
	golangci-lint run
