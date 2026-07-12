package steamcmd

import (
	"fmt"
	"os"
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

	fmt.Println("SteamCMD installed successfully")
	return nil
}

// getExecutablePath returns the path to the SteamCMD executable based on OS
func getExecutablePath(steamcmdPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(steamcmdPath, "steamcmd.exe")
	}
	return filepath.Join(steamcmdPath, "steamcmd.sh")
}
