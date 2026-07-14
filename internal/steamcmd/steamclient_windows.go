//go:build windows

package steamcmd

// EnsureSteamClientLinks is a no-op on Windows: the Windows dedicated server does
// not use the ~/.steam/sdk64 steamclient.so layout that the Linux server needs.
func EnsureSteamClientLinks(steamcmdPath string) error {
	return nil
}
