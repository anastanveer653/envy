// Package ui holds envy's terminal output and prompt helpers.
//
// Status messages go to stderr so that stdout stays clean for data that
// users pipe or capture, such as `envy get` and `envy export`.
package ui

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"golang.org/x/term"
)

var (
	Bold   = color.New(color.Bold).SprintFunc()
	Cyan   = color.New(color.FgCyan, color.Bold).SprintFunc()
	Green  = color.New(color.FgGreen).SprintFunc()
	Red    = color.New(color.FgRed).SprintFunc()
	Yellow = color.New(color.FgYellow).SprintFunc()
	Dim    = color.New(color.Faint).SprintFunc()
)

// ErrNoTTY means a secret had to be read interactively but no terminal is
// attached (for example in CI).
var ErrNoTTY = errors.New("no terminal available for a password prompt; set ENVY_PASSWORD instead")

// DisableColor turns off ANSI colors for the rest of the process.
func DisableColor() { color.NoColor = true }

// Success prints a green check line.
func Success(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Green("✓"), fmt.Sprintf(format, a...))
}

// Warn prints a yellow warning line.
func Warn(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Yellow("!"), fmt.Sprintf(format, a...))
}

// Error prints a red error line.
func Error(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Red("✗"), fmt.Sprintf(format, a...))
}

// IsTerminal reports whether f is attached to a terminal.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// ReadSecret prompts on stderr and reads a line from the controlling
// terminal without echoing it. It works even when stdin is a pipe, so
// `echo value | envy set KEY` can still ask for the master password.
func ReadSecret(prompt string) ([]byte, error) {
	in := os.Stdin
	if !IsTerminal(in) {
		tty, err := openTTY()
		if err != nil {
			return nil, ErrNoTTY
		}
		defer tty.Close()
		in = tty
	}
	fmt.Fprint(os.Stderr, prompt)
	secret, err := term.ReadPassword(int(in.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}
	return secret, nil
}
