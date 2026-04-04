package console

import (
	"fmt"
	"io"
	"os"
)

// Printer writes the minimum console output required by the migrated CLI flow.
type Printer struct {
	// Writer receives rendered console output.
	Writer io.Writer
}

// NewPrinter builds a printer that defaults to stdout.
func NewPrinter(writer io.Writer) *Printer {
	if writer == nil {
		writer = os.Stdout
	}

	return &Printer{Writer: writer}
}

// Print writes text without adding a trailing newline.
func (p *Printer) Print(text string) error {
	_, err := io.WriteString(p.Writer, text)
	return err
}

// PrintLine writes one line terminated by a newline.
func (p *Printer) PrintLine(text string) error {
	_, err := fmt.Fprintln(p.Writer, text)
	return err
}
