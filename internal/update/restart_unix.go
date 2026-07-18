//go:build !windows

package update

import (
	"fmt"
	"os"
)

// restart launches a new instance of the current binary with the same
// arguments and environment on Unix-like systems (Linux, macOS).
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
	return proc.Release()
}
