package steamcmd

import (
	"fmt"
	"os"
	"runtime"
)

// downloadSteamCMD downloads and extracts SteamCMD based on the operating system
func downloadSteamCMD(destPath string) error {
	var downloadURL string
	var isZip bool

	switch runtime.GOOS {
	case "windows":
		downloadURL = steamCMDWindowsURL
		isZip = true
	case "linux":
		downloadURL = steamCMDLinuxURL
		isZip = false
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Download file
	tmpFile, err := downloadFile(downloadURL)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// Extract based on file type
	if isZip {
		return extractZip(tmpFile, destPath)
	}
	return extractTarGz(tmpFile, destPath)
}
