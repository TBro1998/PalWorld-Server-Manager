//go:build windows

package process

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// sysProcAttr returns platform-specific process attributes.
// On Windows we create a new process group so we can signal the whole tree.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// isProcessAlive reports whether a process with the given PID is currently running.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// tasklist filtered by PID; if the PID is present the output contains it.
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH").Output()
	if err != nil {
		return false
	}
	// When no task matches, tasklist prints "INFO: No tasks are running..."
	return !containsNoTasks(out) && len(out) > 0
}

func containsNoTasks(out []byte) bool {
	const marker = "No tasks are running"
	s := string(out)
	for i := 0; i+len(marker) <= len(s); i++ {
		if s[i:i+len(marker)] == marker {
			return true
		}
	}
	return false
}

// killProcess attempts a graceful shutdown of the process tree, escalating to a
// forced kill if the process is still alive after timeout.
func killProcess(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	pidStr := strconv.Itoa(pid)

	// Graceful: taskkill with /T terminates child processes too.
	_ = exec.Command("taskkill", "/PID", pidStr, "/T").Run()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	// Force kill the tree.
	if isProcessAlive(pid) {
		return exec.Command("taskkill", "/F", "/PID", pidStr, "/T").Run()
	}
	return nil
}
