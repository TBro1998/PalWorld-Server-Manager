package update

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Checker queries GitHub for the latest release and caches the result.
type Checker struct {
	repo      string
	buildInfo BuildInfo
	client    *http.Client
}

// NewChecker creates a Checker for the given GitHub repo ("owner/repo") and
// build metadata.
func NewChecker(repo string, info BuildInfo) *Checker {
	return &Checker{
		repo:      repo,
		buildInfo: info,
		client:    newHTTPClient(),
	}
}

// BuildInfo returns the build metadata this checker was created with.
func (c *Checker) BuildInfo() BuildInfo { return c.buildInfo }

// Check performs a live GitHub release lookup, stores the result in the
// in-memory cache, and returns it.  On failure it still stores a CheckResult
// with Err set so the UI can surface the error.
func (c *Checker) Check(ctx context.Context) (*CheckResult, error) {
	now := _now().UTC().Format(time.RFC3339)

	if IsDev(c.buildInfo.Version) {
		r := &CheckResult{
			CurrentVersion: c.buildInfo.Version,
			IsDev:          true,
			HasUpdate:      false,
			CheckedAt:      now,
		}
		CacheResult(r)
		return r, nil
	}

	release, err := FetchLatestRelease(ctx, c.client, c.repo)
	if err != nil {
		r := &CheckResult{
			CurrentVersion: c.buildInfo.Version,
			CheckedAt:      now,
			Err:            err.Error(),
		}
		CacheResult(r)
		return r, err
	}

	asset, ok := SelectAsset(release)
	if !ok {
		err := fmt.Errorf("no release asset for this platform")
		r := &CheckResult{
			CurrentVersion: c.buildInfo.Version,
			LatestVersion:  release.TagName,
			CheckedAt:      now,
			Err:            err.Error(),
		}
		CacheResult(r)
		return r, err
	}

	r := &CheckResult{
		CurrentVersion: c.buildInfo.Version,
		IsDev:          false,
		HasUpdate:      HasUpdate(c.buildInfo.Version, release.TagName),
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
		AssetName:      asset.Name,
		AssetURL:       asset.BrowserDownloadURL,
		AssetSize:      asset.Size,
		CheckedAt:      now,
	}
	CacheResult(r)
	return r, nil
}

// StartBackgroundCheck runs Check in a goroutine, logging any error.  This is
// called once at server startup so the first UI page-load already has a cached
// result without blocking the HTTP listener.
func (c *Checker) StartBackgroundCheck() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := c.Check(ctx); err != nil {
			log.Printf("update: background check failed: %v", err)
		}
	}()
}
