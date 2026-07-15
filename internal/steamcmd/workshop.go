package steamcmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// palworldClientAppID is the Steam App ID that owns Palworld workshop content.
// This is the game *client* app (1623730), NOT the dedicated server app
// (2394010): workshop items are published against the client and downloaded
// with that id even though they are deployed into the server.
// Reference: https://docs.palworldgame.com/settings-and-operation/mod/
const palworldClientAppID = "1623730"

// DownloadWorkshopItem downloads a single Steam Workshop item via SteamCMD and
// returns the directory SteamCMD unpacked it into:
//
//	<steamcmdPath>/steamapps/workshop/content/1623730/<workshopID>
//
// steamUsername selects the Steam login. Palworld is a paid title, so its
// workshop content CANNOT be downloaded with "anonymous" — that only self-updates
// SteamCMD and leaves the content directory empty (confirmed on Windows,
// 2026-07-15). A real account that OWNS Palworld is required. To avoid handling
// passwords or Steam Guard codes, only the username is passed here: the user must
// have completed a one-time interactive `steamcmd +login <user>` beforehand so
// SteamCMD has a cached session this reuses. An empty steamUsername falls back to
// "anonymous" (which will fail for Palworld) so the failure path is explicit.
//
// SteamCMD's stdout/stderr are written to out so callers can persist and stream
// them; out must never be os.Stdout/os.Stderr (that would leak output onto the
// manager's own console). When out is nil, output is discarded — matching
// InstallPalworldServer's contract. The landing directory is verified to exist
// afterwards, so an invalid id / silent login failure becomes an explicit error.
func DownloadWorkshopItem(steamcmdPath, steamUsername, workshopID string, out io.Writer) (string, error) {
	if out == nil {
		out = io.Discard
	}

	steamCmdExe := getExecutablePath(steamcmdPath)
	if _, err := os.Stat(steamCmdExe); os.IsNotExist(err) {
		return "", fmt.Errorf("SteamCMD not found at: %s", steamCmdExe)
	}

	login := strings.TrimSpace(steamUsername)
	anonymous := login == ""
	if anonymous {
		login = "anonymous"
	}

	// No +force_install_dir: workshop content always lands under SteamCMD's own
	// steamapps/workshop tree, regardless of any install-dir override.
	args := []string{
		"+login", login,
		"+workshop_download_item", palworldClientAppID, workshopID,
		"+quit",
	}

	fmt.Fprintf(out, "==> Downloading workshop item %s (app %s) as %s...\n", workshopID, palworldClientAppID, login)

	cmd := exec.Command(steamCmdExe, args...)
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(out, "==> Download failed: %v\n", err)
		return "", fmt.Errorf("SteamCMD workshop download failed for %s: %w", workshopID, err)
	}

	dir := filepath.Join(steamcmdPath, "steamapps", "workshop", "content", palworldClientAppID, workshopID)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		fmt.Fprintf(out, "==> Downloaded content not found at %s: %s\n", dir, loginHint(anonymous))
		return "", fmt.Errorf("workshop item %s not downloaded: %s", workshopID, loginHint(anonymous))
	}

	fmt.Fprintf(out, "==> Download complete: %s\n", dir)
	return dir, nil
}

// loginHint returns an actionable message for a download that produced no
// content, tailored to whether a Steam username was configured.
func loginHint(anonymous bool) string {
	if anonymous {
		return "anonymous login cannot download Palworld workshop content; set steam_username to an account that owns Palworld and run `steamcmd +login <user>` once to cache its session"
	}
	return "content missing after download — verify the Workshop ID is valid and that `steamcmd +login <user>` was run once interactively to cache the session (this tool does not handle passwords or Steam Guard codes)"
}
