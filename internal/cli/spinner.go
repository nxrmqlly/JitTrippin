package cli

import (
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/mattn/go-isatty"
)

const (
	spinnerChars    = 11
	spinnerInterval = 120 * time.Millisecond
	spinnerColor    = "fgCyan"
)

// Spinner renders an indeterminate progress indicator on a single line.
type Spinner struct {
	spin *spinner.Spinner
}

func NewSpinner(w io.Writer, msg string) *Spinner {
	opts := []spinner.Option{}
	if f, ok := w.(*os.File); ok {
		opts = append(opts, spinner.WithWriterFile(f))
	}
	s := spinner.New(spinner.CharSets[spinnerChars], spinnerInterval, opts...)
	s.Color(spinnerColor)
	s.Prefix = msg + " "
	return &Spinner{spin: s}
}

// Start begins animating. It is a no-op if w is not a terminal.
func (s *Spinner) Start() {
	if s.spin.Writer == nil || !isTerminal(s.spin.Writer) {
		return
	}
	s.spin.Start()
}

// Stop clears the spinner line and restores the cursor.
func (s *Spinner) Stop() {
	s.spin.Stop()
}

// RunWithProgress runs fn while showing an indeterminate progress indicator
// labelled msg, in the style of `gh`. The indicator is hidden when stderr is
// not a terminal.
func RunWithProgress(msg string, fn func() error) error {
	s := NewSpinner(os.Stderr, msg)
	s.Start()
	defer s.Stop()
	return fn()
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// colorEnabled reports whether ANSI colors should be emitted on stdout,
// honouring the NO_COLOR convention.
func colorEnabled() bool {
	return isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == ""
}

const (
	escReset = "\x1b[0m"
	escRed   = "\x1b[31m"
	escGreen = "\x1b[32m"
	escGray  = "\x1b[90m"
)

func paint(code, s string) string {
	if !colorEnabled() {
		return s
	}
	return code + s + escReset
}

func red(s string) string   { return paint(escRed, s) }
func green(s string) string { return paint(escGreen, s) }
func gray(s string) string  { return paint(escGray, s) }

func successIcon() string { return green("✓") }
func failureIcon() string { return red("X") }
