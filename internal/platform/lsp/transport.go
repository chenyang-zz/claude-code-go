package lsp

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// StdioTransport manages an LSP server process and provides io.Reader/io.Writer
// interfaces for JSON-RPC message exchange over stdin/stdout.
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	// reader wraps stdout for framed message reading.
	reader *bufio.Reader
}

// Start launches the LSP server process and connects to its stdio pipes.
func (t *StdioTransport) Start(command string, args ...string) error {
	t.cmd = exec.Command(command, args...)

	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("lsp transport: stdin pipe: %w", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("lsp transport: stdout pipe: %w", err)
	}

	// Capture stderr for diagnostics.
	t.cmd.Stderr = &stderrLogger{prefix: command}

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("lsp transport: start process: %w", err)
	}

	t.reader = bufio.NewReader(t.stdout)
	logger.DebugCF("lsp.transport", "LSP server started", map[string]any{
		"command": command,
	})
	return nil
}

// Writer returns the stdin pipe writer for sending messages to the server.
func (t *StdioTransport) Writer() io.Writer {
	return t.stdin
}

// Reader returns the buffered stdout reader for receiving messages.
func (t *StdioTransport) Reader() *bufio.Reader {
	return t.reader
}

// IsRunning reports whether the server process is still alive.
func (t *StdioTransport) IsRunning() bool {
	if t.cmd == nil || t.cmd.Process == nil {
		return false
	}
	return t.cmd.ProcessState == nil || !t.cmd.ProcessState.Exited()
}

// Close sends a shutdown request, then forces the process to exit if it does
// not terminate gracefully.
func (t *StdioTransport) Close() error {
	if t.stdin != nil {
		t.stdin.Close()
		t.stdin = nil
	}
	if t.cmd != nil && t.cmd.Process != nil {
		if err := t.cmd.Process.Kill(); err != nil {
			logger.DebugCF("lsp.transport", "process kill error", map[string]any{
				"error": err.Error(),
			})
		}
		return t.cmd.Wait()
	}
	return nil
}

// stderrLogger writes LSP server stderr output to the debug log.
type stderrLogger struct {
	prefix string
}

func (l *stderrLogger) Write(p []byte) (int, error) {
	logger.DebugCF("lsp.server", "stderr", map[string]any{
		"server": l.prefix,
		"output": string(p),
	})
	return len(p), nil
}
