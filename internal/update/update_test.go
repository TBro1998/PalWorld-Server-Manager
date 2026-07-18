package update

import (
	"testing"
)

func TestIsDev(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"dev", true},
		{"", true},
		{"not-a-version", true},
		{"v0.6.7", false},
		{"v1.0.0", false},
		{"v0.0.1", false},
		{"v1.2.3-beta.1", false},
	}
	for _, c := range cases {
		got := IsDev(c.v)
		if got != c.want {
			t.Errorf("IsDev(%q) = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestHasUpdate(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
	}{
		{"v0.6.7", "v0.6.8", true},
		{"v0.6.7", "v0.7.0", true},
		{"v0.6.7", "v1.0.0", true},
		// Patch version with many digits — pure string compare would fail here
		{"v0.9.0", "v0.10.0", true},
		{"v0.6.7", "v0.6.7", false},  // same version
		{"v0.6.8", "v0.6.7", false},  // older release
		{"dev", "v1.0.0", false},     // dev build never gets update
		{"v1.0.0", "dev", false},     // invalid latest
		{"", "v1.0.0", false},        // empty current
		{"not-semver", "v1.0.0", false},
	}
	for _, c := range cases {
		got := HasUpdate(c.current, c.latest)
		if got != c.want {
			t.Errorf("HasUpdate(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
	}{
		{"windows", "amd64", "psm_windows_amd64.exe"},
		{"linux", "amd64", "psm_linux_amd64"},
		{"darwin", "amd64", ""},
		{"linux", "arm64", ""},
		{"windows", "386", ""},
	}
	for _, c := range cases {
		got := AssetName(c.goos, c.goarch)
		if got != c.want {
			t.Errorf("AssetName(%q, %q) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestResolveDownloadURL(t *testing.T) {
	raw := "https://github.com/owner/repo/releases/download/v1.0/bin"
	cases := []struct {
		mirror string
		want   string
	}{
		{"", raw},
		{"   ", raw},
		{"https://ghproxy.com", "https://ghproxy.com/" + raw},
		{"https://ghproxy.com/", "https://ghproxy.com/" + raw}, // trailing slash normalised
		{"https://mirror.example.com/proxy", "https://mirror.example.com/proxy/" + raw},
	}
	for _, c := range cases {
		got := ResolveDownloadURL(raw, c.mirror)
		if got != c.want {
			t.Errorf("ResolveDownloadURL(raw, %q)\n  got  %q\n  want %q", c.mirror, got, c.want)
		}
	}
}

func TestReplaceAssetFilename(t *testing.T) {
	cases := []struct {
		url  string
		name string
		want string
	}{
		{
			"https://github.com/o/r/releases/download/v1/psm_linux_amd64",
			"checksums.txt",
			"https://github.com/o/r/releases/download/v1/checksums.txt",
		},
		{
			"https://github.com/o/r/releases/download/v1/psm_windows_amd64.exe",
			"checksums.txt",
			"https://github.com/o/r/releases/download/v1/checksums.txt",
		},
		{"no-slash-url", "checksums.txt", ""},
	}
	for _, c := range cases {
		got := replaceAssetFilename(c.url, c.name)
		if got != c.want {
			t.Errorf("replaceAssetFilename(%q, %q) = %q, want %q", c.url, c.name, got, c.want)
		}
	}
}
