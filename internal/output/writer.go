package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// Mode controls JSON formatting behavior.
type Mode int

const (
	ModeAuto    Mode = iota // Detect TTY: pretty if terminal, compact if piped
	ModePretty              // Force indented JSON
	ModeCompact             // Force single-line JSON
	ModeRaw                 // Pass through raw bytes (for --raw flag)
)

// Printer manages output formatting.
type Printer struct {
	stdout io.Writer
	stderr io.Writer
	mode   Mode
	quiet  bool
}

// NewPrinter creates a Printer.
func NewPrinter(stdout, stderr io.Writer, mode Mode, quiet bool) *Printer {
	if quiet {
		color.NoColor = true
	}
	return &Printer{
		stdout: stdout,
		stderr: stderr,
		mode:   mode,
		quiet:  quiet,
	}
}

// JSON writes v as JSON to stdout.
func (p *Printer) JSON(v interface{}) error {
	var data []byte
	var err error

	switch p.effectiveMode() {
	case ModePretty:
		data, err = json.MarshalIndent(v, "", "  ")
	default:
		data, err = json.Marshal(v)
	}
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	data = append(data, '\n')
	_, err = p.stdout.Write(data)
	return err
}

// RawJSON writes pre-encoded JSON bytes to stdout.
func (p *Printer) RawJSON(data []byte) error {
	if p.effectiveMode() == ModePretty {
		var v interface{}
		if err := json.Unmarshal(data, &v); err == nil {
			return p.JSON(v)
		}
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	_, err := p.stdout.Write(data)
	return err
}

// Error writes an error message to stderr.
func (p *Printer) Error(format string, args ...interface{}) {
	if p.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(p.stderr, "%s %s\n", color.RedString("ebcli:"), msg)
}

// Warn writes a warning message to stderr.
func (p *Printer) Warn(format string, args ...interface{}) {
	if p.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(p.stderr, "%s %s\n", color.YellowString("ebcli:"), msg)
}

// Info writes an informational message to stderr.
func (p *Printer) Info(format string, args ...interface{}) {
	if p.quiet {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(p.stderr, "%s %s\n", color.CyanString("ebcli:"), msg)
}

// IsRaw returns true if the output mode is raw.
func (p *Printer) IsRaw() bool {
	return p.mode == ModeRaw
}

func (p *Printer) effectiveMode() Mode {
	if p.mode != ModeAuto {
		return p.mode
	}
	if f, ok := p.stdout.(*os.File); ok && isTerminal(f) {
		return ModePretty
	}
	return ModeCompact
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// ModeFromFlags converts CLI flag values to a Mode.
// Priority: raw > compact > pretty > auto.
func ModeFromFlags(pretty, compact, raw bool) Mode {
	switch {
	case raw:
		return ModeRaw
	case compact:
		return ModeCompact
	case pretty:
		return ModePretty
	default:
		return ModeAuto
	}
}
