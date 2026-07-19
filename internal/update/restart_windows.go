//go:build windows

package update

import (
	"fmt"
	"os"
	"syscall"
)

// Windows process creation flags used to decouple the child from the parent's
// console session.  Defined here to avoid an indirect dependency on
// golang.org/x/sys/windows just for two constants.
const (
	// createNewProcessGroup isolates the child's signal handling so a
	// Ctrl+C / Ctrl+Break aimed at the parent does not propagate.
	createNewProcessGroup = 0x00000200
	// createNewConsole allocates a new console window for the child process.
	// This replaces the previous DETACHED_PROCESS approach: detached processes
	// have no console at all, whereas CREATE_NEW_CONSOLE gives the restarted
	// binary its own visible window — matching the behaviour of the original
	// process that was launched by the user.
	createNewConsole = 0x00000010
)

// restart launches a new instance of the current binary with the same
// arguments and environment on Windows.
//
// The child is started with DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP so it
// is completely independent of the parent's console and process group.
// Inheriting the parent's stdio handles (the previous behaviour) ties the child
// to the parent's console window: when the parent exits the console closes and
// the OS delivers CTRL_CLOSE_EVENT to every process attached to it, including
// the newly spawned child.  Detaching avoids that race.
func restart() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	attr := &os.ProcAttr{
		// Do not inherit stdio — the child runs detached and will open its own
		// handles (e.g. log file) during initialisation.
		Files: []*os.File{nil, nil, nil},
		Env:   os.Environ(),
		Sys: &syscall.SysProcAttr{
			CreationFlags: createNewConsole | createNewProcessGroup,
		},
	}

	proc, err := os.StartProcess(exePath, os.Args, attr)
	if err != nil {
		return fmt.Errorf("start new process: %w", err)
	}
	// Release OS resources for the child — we do not wait for it.
	return proc.Release()
}
