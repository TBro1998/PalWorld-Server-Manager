package process

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// serverExecutable returns the path to the Palworld dedicated server launcher
// for the current platform, verifying that the file exists.
//
//	Windows: <installPath>/PalServer.exe
//	Linux:   <installPath>/PalServer.sh
func serverExecutable(installPath string) (string, error) {
	var name string
	if runtime.GOOS == "windows" {
		name = "PalServer.exe"
	} else {
		name = "PalServer.sh"
	}

	exe := filepath.Join(installPath, name)
	if _, err := os.Stat(exe); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("server executable not found at %s (is the server installed?)", exe)
		}
		return "", fmt.Errorf("failed to stat server executable %s: %w", exe, err)
	}
	return exe, nil
}
