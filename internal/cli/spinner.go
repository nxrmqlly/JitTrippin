package cli

import (
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
)

// Spinner renders an indeterminate progress indicator on a single line.
type Spinner struct {
	spin *spinner.Spinner
}

func NewSpinner(w io.Writer, msg string) *Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = w
	s.Suffix = " " + msg
	return &Spinner{spin: s}
}

// Start begins animating. It is a no-op if w is not a terminal.
func (s *Spinner) Start() {
	if s.spin.Writer == nil || !isTerminal(s.spin.Writer) {
		return
	}
	s.spin.Start()
}

// Stop clears the spinner line.
func (s *Spinner) Stop() {
	s.spin.Stop()
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
