package cmd

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/briandowns/spinner"
)

// newSpinner creates a consistently configured spinner writing to w.
func newSpinner(w io.Writer, suffix string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = w
	s.Suffix = fmt.Sprintf(" %s", suffix)
	return s
}

// openBrowser opens path in the OS default handler.
func openBrowser(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
