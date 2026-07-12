package steamcmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// InstallPalworldServer downloads and installs Palworld dedicated server using SteamCMD.
// App ID for Palworld dedicated server: 2394010
// Reference: https://docs.palworldgame.com/getting-started/deploy-dedicated-server
//
// SteamCMD's stdout/stderr are written to out so callers can persist them to a
// per-server log file and stream them to the frontend. out must never be
// os.Stdout/os.Stderr (that would leak install output onto the manager's own
// console). When out is nil, output is discarded.
func InstallPalworldServer(installPath string, steamCmdPath string, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}

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

	// Execute SteamCMD, routing its output to the caller-supplied writer.
	fmt.Fprintf(out, "==> Installing Palworld dedicated server (app 2394010)...\n")

	cmd := exec.Command(steamCmdExe, args...)
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(out, "==> Installation failed: %v\n", err)
		return fmt.Errorf("SteamCMD installation failed: %w", err)
	}

	fmt.Fprintf(out, "==> Installation complete\n")
	return nil
}
