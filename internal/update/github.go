package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	githubAPIBase = "https://api.github.com"
	userAgent     = "PalWorld-Server-Manager"
	httpTimeout   = 15 * time.Second
)

// ReleaseInfo holds the fields we care about from a GitHub release.
type ReleaseInfo struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	PublishedAt string  `json:"published_at"`
	Assets      []Asset `json:"assets"`
}

// Asset represents one file attached to a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// newHTTPClient returns an HTTP client configured for GitHub API calls.
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: httpTimeout}
}

// FetchLatestRelease queries the GitHub Releases API for the latest published
// release of the given repository (format: "owner/repo").
func FetchLatestRelease(ctx context.Context, client *http.Client, repo string) (*ReleaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBase, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %d for %s", resp.StatusCode, url)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &release, nil
}
