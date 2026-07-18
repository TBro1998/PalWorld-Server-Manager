//go:build windows

package update

import (
	"fmt"
	"os"
)

// restart launches a new instance of the current binary with the same
// arguments and environment on Windows.  On Windows we cannot simply
// os.StartProcess with inherited stdio because the current process may own a
// console window; instead we let the OS spawn a new independent process.
func restart() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	attr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Env:   os.Environ(),
	}

	proc, err := os.StartProcess(exePath, os.Args, attr)
	if err != nil {
		return fmt.Errorf("start new process: %w", err)
	}
	// Detach: we do not wait for the new process; it will manage its own
	// lifecycle.  Release releases the OS resources associated with the Proc.
	return proc.Release()
}
