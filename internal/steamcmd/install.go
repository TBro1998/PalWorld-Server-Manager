package steamcmd

import (
	"fmt"
	"os"
	"os/exec"
)

// InstallPalworldServer downloads and installs Palworld dedicated server using SteamCMD
// App ID for Palworld dedicated server: 2394010
// Reference: https://docs.palworldgame.com/getting-started/deploy-dedicated-server
func InstallPalworldServer(installPath string, steamCmdPath string) error {
	// Create install directory if it doesn't exist
	if err := os.MkdirAll(installPath, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Get SteamCMD executable path (defined in steamcmd.go)
	steamCmdExe := getExecutablePath(steamCmdPath)
	if _, err := os.Stat(steamCmdExe); os.IsNotExist(err) {
		return fmt.Errorf("SteamCMD not found at: %s", steamCmdExe)
	}

	// Prepare SteamCMD command
	// NOTE: +force_install_dir MUST come before +login, otherwise SteamCMD fails with
	// "Please use force_install_dir before logon!" and aborts the app install.
	// +force_install_dir <path> - Set install directory (must precede login)
	// +login anonymous - Login as anonymous user
	// +app_update 2394010 validate - Download/update Palworld dedicated server with validation
	// +quit - Exit SteamCMD after completion
	args := []string{
		"+force_install_dir", installPath,
		"+login", "anonymous",
		"+app_update", "2394010", "validate",
		"+quit",
	}

	// Execute SteamCMD
	cmd := exec.Command(steamCmdExe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SteamCMD installation failed: %w", err)
	}

	return nil
}
