// Package update implements self-update for the PalWorld Server Manager binary.
// It queries the GitHub Releases API, compares semver tags, downloads the
// platform-appropriate asset, verifies its SHA-256 checksum (when a checksums
// file is present in the release), applies the binary replacement atomically
// via minio/selfupdate, and re-launches the new binary in-place.
package update

import (
	"sync/atomic"
	"time"
)

// BuildInfo carries the version metadata injected at link time via -ldflags.
// main.go creates one from its package-level Version/BuildTime/GitCommit vars
// and passes it down through server.New → api.NewRouter.
type BuildInfo struct {
	Version   string
	BuildTime string
	GitCommit string
}

// CheckResult is the outcome of one GitHub release check, cached in memory so
// the UI can read it without hitting GitHub on every page load.
type CheckResult struct {
	// CurrentVersion is the running binary's version tag (e.g. "v0.6.7").
	CurrentVersion string `json:"currentVersion"`
	// IsDev is true when the binary was built without a real version tag.
	IsDev bool `json:"isDev"`
	// HasUpdate is true when a newer release exists on GitHub.
	HasUpdate bool `json:"hasUpdate"`
	// LatestVersion is the tag_name of the latest GitHub release.
	LatestVersion string `json:"latestVersion,omitempty"`
	// ReleaseNotes is the release body text from GitHub.
	ReleaseNotes string `json:"releaseNotes,omitempty"`
	// AssetName is the filename of the binary asset for this platform.
	AssetName string `json:"assetName,omitempty"`
	// AssetURL is the raw GitHub browser_download_url for the asset.
	AssetURL string `json:"assetURL,omitempty"`
	// AssetSize is the asset size in bytes reported by the GitHub API.
	AssetSize int64 `json:"assetSize,omitempty"`
	// CheckedAt is when this result was computed (RFC 3339).
	CheckedAt string `json:"checkedAt,omitempty"`
	// Err is non-empty when the check failed; other fields may be zero.
	Err string `json:"err,omitempty"`
}

// resultStore holds the most recent check result.  It is written once after
// the startup background check and again on every manual check; reads are
// always safe from any goroutine.
var resultStore atomic.Pointer[CheckResult]

// CacheResult stores r as the latest cached check result.
func CacheResult(r *CheckResult) { resultStore.Store(r) }

// Cached returns the most recently stored check result, or nil if no check
// has been completed yet.
func Cached() *CheckResult { return resultStore.Load() }

// UpdateStreamID is the sentinel serverID used with StreamManager for update
// progress messages.  Real server IDs start at 1 (auto-increment), so 0 is
// safe to use as a non-colliding sentinel.
const UpdateStreamID int64 = 0

// ProgressFunc is called during a download to report progress and log lines.
// pct is 0-100; msg is a human-readable status string.
type ProgressFunc func(pct int, msg string)

// tickDuration is used in tests to override time.Since / time.Sleep behaviour.
var _now = time.Now

// UpdatePhase describes the lifecycle state of a self-update operation.
type UpdatePhase string

const (
	PhaseIdle        UpdatePhase = "idle"
	PhaseDownloading UpdatePhase = "downloading"
	PhaseRestarting  UpdatePhase = "restarting"
	PhaseError       UpdatePhase = "error"
)

// UpdateStatus is the current in-progress update state.  It is stored
// atomically so the frontend can poll GET /api/system/update/status on page
// load and restore progress UI after a navigation or browser refresh.
type UpdateStatus struct {
	Phase UpdatePhase `json:"phase"`
	Pct   int         `json:"pct"`
	Msg   string      `json:"msg"`
	Err   string      `json:"err,omitempty"`
}

var statusStore atomic.Value // *UpdateStatus

func init() { statusStore.Store(&UpdateStatus{Phase: PhaseIdle}) }

// SetStatus atomically replaces the current update status.
func SetStatus(s UpdateStatus) { statusStore.Store(&s) }

// Status returns the current update status.
func Status() UpdateStatus {
	if v := statusStore.Load(); v != nil {
		return *v.(*UpdateStatus)
	}
	return UpdateStatus{Phase: PhaseIdle}
}
