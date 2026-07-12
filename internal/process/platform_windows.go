//go:build windows

package process

import (
	"os/exec"
	"strconv"
	"strings"
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
//
// It queries tasklist in CSV format and checks whether the output contains the
// quoted PID. A matching process produces a CSV row like
// `"image.exe","<pid>","Console",...`; when no process matches, tasklist emits a
// localized info message that will not contain `"<pid>"`. Matching on the quoted
// PID keeps this locale-independent (non-English Windows included).
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command(
		"tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH", "/FO", "CSV",
	).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), `"`+strconv.Itoa(pid)+`"`)
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
