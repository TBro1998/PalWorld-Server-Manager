package steamcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	// SteamCMD download URLs
	steamCMDWindowsURL = "https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip"
	steamCMDLinuxURL   = "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz"
)

// CheckAndInstall checks if SteamCMD exists at the given path, and downloads it if not
func CheckAndInstall(steamcmdPath string) error {
	// Check if SteamCMD executable exists
	execPath := getExecutablePath(steamcmdPath)
	if _, err := os.Stat(execPath); err == nil {
		// SteamCMD already exists
		fmt.Printf("SteamCMD found at: %s\n", execPath)
		return nil
	}

	fmt.Printf("SteamCMD not found at: %s\n", execPath)
	fmt.Println("Downloading SteamCMD...")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(steamcmdPath, 0755); err != nil {
		return fmt.Errorf("failed to create steamcmd directory: %w", err)
	}

	// Download and extract SteamCMD
	if err := downloadSteamCMD(steamcmdPath); err != nil {
		return fmt.Errorf("failed to download steamcmd: %w", err)
	}

	fmt.Println("SteamCMD downloaded, running initial update...")

	// A freshly downloaded SteamCMD is only a bootstrap: on Windows it is a bare
	// steamcmd.exe and on Linux a shell wrapper. It must be run once so it can
	// self-update and unpack its full package before it can install game servers.
	if err := runInitialUpdate(execPath); err != nil {
		return fmt.Errorf("failed to run initial steamcmd update: %w", err)
	}

	fmt.Println("SteamCMD installed successfully")
	return nil
}

// runInitialUpdate runs SteamCMD once with +quit so it bootstraps and applies
// its own update. Output is streamed to the manager console so the (potentially
// slow) first-run download is visible.
func runInitialUpdate(execPath string) error {
	cmd := exec.Command(execPath, "+quit")
	cmd.Dir = filepath.Dir(execPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getExecutablePath returns the path to the SteamCMD executable based on OS
func getExecutablePath(steamcmdPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(steamcmdPath, "steamcmd.exe")
	}
	return filepath.Join(steamcmdPath, "steamcmd.sh")
}
