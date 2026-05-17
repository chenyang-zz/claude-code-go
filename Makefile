.PHONY: build test test-tui test-all tui-deps tui-run tui-build clean run-tui run run-tui-debug

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

# Run TUI mode (set TUI_TMUX=0 to disable tmux split-pane)
run-tui:
	chmod +x scripts/run-tui.sh && scripts/run-tui.sh

# Run TUI mode with full frontend debug logging
run-tui-debug:
	chmod +x scripts/run-tui.sh && TUI_DEBUG=1 TUI_DEBUG_LEVEL=trace TUI_DEBUG_MODULE=ws,render,state TUI_DEBUG_CONTENT=full TUI_DEBUG_RESET_ON_START=1 scripts/run-tui.sh

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
