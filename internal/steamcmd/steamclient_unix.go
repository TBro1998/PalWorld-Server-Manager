//go:build !windows

package steamcmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureSteamClientLinks wires up the Steam client library symlinks the Palworld
// Linux dedicated server expects.
//
// The Linux server looks up steamclient.so under $HOME/.steam/sdk64 (and sdk32),
// but SteamCMD unpacks that library into its own linux64/linux32 directories.
// Without these links the server exits early or spams "steamclient.so missing"
// errors, so this is required for a working Linux install. On Windows this is a
// no-op (see steamclient_windows.go).
//
// It is best-effort and safe to call repeatedly: existing correct links are kept,
// stale ones are recreated. Symlinks may point at a target that does not exist
// yet (SteamCMD finishes unpacking on its first real run), and will resolve once
// the file appears.
func EnsureSteamClientLinks(steamcmdPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}

	absSteamcmd, err := filepath.Abs(steamcmdPath)
	if err != nil {
		return fmt.Errorf("resolve steamcmd path: %w", err)
	}

	links := []struct{ dir, target string }{
		{filepath.Join(home, ".steam", "sdk64"), filepath.Join(absSteamcmd, "linux64", "steamclient.so")},
		{filepath.Join(home, ".steam", "sdk32"), filepath.Join(absSteamcmd, "linux32", "steamclient.so")},
	}

	for _, l := range links {
		if err := os.MkdirAll(l.dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", l.dir, err)
		}
		link := filepath.Join(l.dir, "steamclient.so")

		// Keep the link if it already points at the right target.
		if cur, err := os.Readlink(link); err == nil && cur == l.target {
			continue
		}
		// Remove whatever is there (wrong link or regular file); ignore "not exist".
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale link %s: %w", link, err)
		}
		if err := os.Symlink(l.target, link); err != nil {
			return fmt.Errorf("symlink %s -> %s: %w", link, l.target, err)
		}
	}

	return nil
}
