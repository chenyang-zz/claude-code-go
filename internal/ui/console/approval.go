package console

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
)

// ApprovalRenderer renders the minimum CLI approval prompt and captures one approve/deny response from stdin.
type ApprovalRenderer struct {
	// Printer owns the caller-facing writes for approval prompts.
	Printer *Printer
	// Reader provides the input stream consumed for approval decisions.
	Reader io.Reader
}

// NewApprovalRenderer builds an approval renderer with optional explicit IO dependencies.
func NewApprovalRenderer(printer *Printer, reader io.Reader) *ApprovalRenderer {
	if printer == nil {
		printer = NewPrinter(nil)
	}
	if reader == nil {
		reader = os.Stdin
	}
	return &ApprovalRenderer{
		Printer: printer,
		Reader:  reader,
	}
}

// Prompt renders one approval prompt and returns a minimal approve/deny decision.
func (r *ApprovalRenderer) Prompt(ctx context.Context, prompt approval.Prompt) (approval.Response, error) {
	_ = ctx

	if err := r.Printer.PrintLine(prompt.Title); err != nil {
		return approval.Response{}, err
	}
	if strings.TrimSpace(prompt.Body) != "" {
		if err := r.Printer.PrintLine(prompt.Body); err != nil {
			return approval.Response{}, err
		}
	}
	if err := r.Printer.Print("Approve? [y/N]: "); err != nil {
		return approval.Response{}, err
	}

	reader := bufio.NewReader(r.Reader)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return approval.Response{}, err
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "y" || answer == "yes" {
		return approval.Response{Approved: true}, nil
	}

	return approval.Response{
		Approved: false,
		Reason:   fmt.Sprintf("Approval denied for %s.", strings.TrimSpace(prompt.Title)),
	}, nil
}
