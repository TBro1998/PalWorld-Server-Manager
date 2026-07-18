package update

import (
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

// normalize ensures v has a "v" prefix so it is valid input for
// golang.org/x/mod/semver.  Tags from GitHub already start with "v"; local
// dev builds may not.
func normalize(v string) string {
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// IsDev reports whether v is the placeholder "dev" string or any string that
// is not a valid semantic version.  When IsDev returns true the binary was
// built outside the release workflow and update checks are skipped.
func IsDev(v string) bool {
	if v == "dev" || v == "" {
		return true
	}
	return !semver.IsValid(normalize(v))
}

// HasUpdate reports whether latestTag is a higher version than currentTag.
// Returns false (never blocks, never errors) when either tag is a dev/invalid
// version so that development builds never prompt for updates.
func HasUpdate(currentTag, latestTag string) bool {
	if IsDev(currentTag) || IsDev(latestTag) {
		return false
	}
	return semver.Compare(normalize(latestTag), normalize(currentTag)) > 0
}

// AssetName returns the expected filename of the release binary asset for the
// given GOOS/GOARCH pair.  Returns "" when the platform is not covered by the
// release workflow (i.e. self-update is not supported on that platform).
func AssetName(goos, goarch string) string {
	switch fmt.Sprintf("%s/%s", goos, goarch) {
	case "windows/amd64":
		return "palsm_windows_amd64.exe"
	case "linux/amd64":
		return "palsm_linux_amd64"
	default:
		return ""
	}
}

// CurrentAssetName returns the asset name for the platform this binary is
// running on.
func CurrentAssetName() string {
	return AssetName(runtime.GOOS, runtime.GOARCH)
}

// SelectAsset finds the release asset that matches the current platform.
// Returns (asset, true) on success and (Asset{}, false) when no matching
// asset is found in the release.
func SelectAsset(release *ReleaseInfo) (Asset, bool) {
	name := CurrentAssetName()
	if name == "" {
		return Asset{}, false
	}
	for _, a := range release.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// ResolveDownloadURL applies the optional download mirror prefix to a raw
// GitHub asset URL.  When mirror is blank (or whitespace only) the original
// URL is returned unchanged.
//
// Mirror convention: the mirror service proxies the full original URL as a
// path component.  Example:
//
//	mirror  = "https://ghproxy.com"
//	rawURL  = "https://github.com/owner/repo/releases/download/v1.0/bin"
//	result  = "https://ghproxy.com/https://github.com/owner/repo/releases/download/v1.0/bin"
func ResolveDownloadURL(rawURL, mirror string) string {
	mirror = strings.TrimSpace(mirror)
	if mirror == "" {
		return rawURL
	}
	return strings.TrimRight(mirror, "/") + "/" + rawURL
}
