//go:build !windows

package process

import (
	"syscall"
	"time"
)

// sysProcAttr returns platform-specific process attributes.
// On Unix we start the server in its own process group (Setpgid) so we can
// signal the entire group and reliably reap child processes.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// isProcessAlive reports whether a process with the given PID is currently running.
// Signal 0 performs error checking without actually sending a signal.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists but we lack permission to signal it.
	return err == syscall.EPERM
}

// killProcess attempts a graceful shutdown via SIGTERM to the process group,
// escalating to SIGKILL if the process is still alive after timeout.
func killProcess(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}

	// Negative PID targets the whole process group created via Setpgid.
	pgid := -pid

	// Graceful termination.
	if err := syscall.Kill(pgid, syscall.SIGTERM); err != nil {
		// Fall back to signaling just the process if the group send fails.
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	// Force kill.
	if isProcessAlive(pid) {
		if err := syscall.Kill(pgid, syscall.SIGKILL); err != nil {
			return syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	return nil
}
