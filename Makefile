.PHONY: build test test-tui test-all tui-deps tui-run tui-build clean run-tui run

# Go binary name
BINARY=cc

# Build the Go binary
build:
	go build -o $(BINARY) ./cmd/cc/.

# Run tests
test:
	go test ./...

# Run only TUI package tests
test-tui:
	go test ./internal/ui/tui/... -v -count=1

# Run all tests (Go + TUI)
test-all: test
	cd tui && bunx tsc --noEmit

# Install TUI frontend dependencies
tui-deps:
	cd tui && bun install

# Run TUI frontend in dev mode (requires `run-tui` or `cc --tui` already running)
tui-run:
	cd tui && bun run src/index.tsx --port=$${TUI_PORT:-8080}

# Build TUI standalone binary (requires bun)
tui-build:
	cd tui && bun build --compile src/index.tsx --outfile ../tui-bin

# Clean build artifacts
clean:
	rm -f $(BINARY) tui-bin
	go clean

# Run TUI mode: Go backend (background) + TUI frontend (foreground, with PTY)
run-tui:
	chmod +x scripts/run-tui.sh && scripts/run-tui.sh

# Watch Go logs from the TUI session (run in another terminal)
tui-logs:
	tail -f /tmp/cc-tui-port.log

# Run in console mode (default)
run:
	go run ./cmd/cc/.

# Format Go code
fmt:
	go fmt ./...

# TypeScript type check
tsc:
	cd tui && bunx tsc --noEmit
