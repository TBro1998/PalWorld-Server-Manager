//go:build windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// launchTarget resolves the executable to spawn and the working directory to
// spawn it in, chosen so the server's stdout/stderr can be captured directly.
//
// The root PalServer.exe is only a launcher: it starts the real server
// (PalServer-Win64-Shipping-Cmd.exe) in a separate console whose output we
// cannot capture. So we run that console binary directly from its own
// Pal\Binaries\Win64 directory (where steam_appid.txt and the required DLLs
// live) and pipe its stdout/stderr. Falls back to the launcher only if the
// direct binary is missing (log capture will then be limited).
func launchTarget(installPath string) (exe, workDir string, err error) {
	cmdExe := filepath.Join(installPath, "PalServer.exe")
	// cmdExe := filepath.Join(installPath, "Pal", "Binaries", "Win64", "PalServer-Win64-Shipping-Cmd.exe")
	if _, statErr := os.Stat(cmdExe); statErr == nil {
		return cmdExe, filepath.Dir(cmdExe), nil
	}
	launcher := filepath.Join(installPath, "PalServer.exe")
	if _, statErr := os.Stat(launcher); statErr != nil {
		return "", "", fmt.Errorf("server executable not found under %s (is the server installed?)", installPath)
	}
	return launcher, installPath, nil
}

// logArgs returns the Unreal Engine flags appended at launch that force the
// dedicated server to write its runtime log to the (redirected) stdout we
// capture. Without -stdout, UE on Windows writes only to its console window and
// log devices, never the redirected stdout handle, so nothing is captured.
// -FullStdOutLogOutput routes the complete log (Palworld writes no log file by
// default) and -UTF8Output prevents UTF-16 mojibake in the captured stream.
// Added at start time only; never persisted to the user launch configuration.
func logArgs() []string {
	return []string{"-log", "-stdout", "-FullStdOutLogOutput", "-UTF8Output"}
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
