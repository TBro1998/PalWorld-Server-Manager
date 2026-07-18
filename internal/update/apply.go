package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/minio/selfupdate"
)

// Apply downloads the asset described by result, optionally verifies its
// SHA-256 against a checksums file in the same release, replaces the running
// binary atomically via minio/selfupdate, and then re-execs the new binary.
//
// progress is called repeatedly during the download with the current percentage
// (0-100) and a status message.  It is also called with pct=-1 for non-progress
// log lines (checksum verification, apply status, etc.).
//
// onRestarting is called right before the process exits, giving the caller a
// chance to flush a "restarting" SSE event to connected clients before the
// HTTP server dies.  It must not block for more than a few hundred milliseconds.
//
// On any failure before the binary has been replaced, Apply returns an error
// and the original binary remains intact.  After a successful selfupdate.Apply
// the function initiates re-exec and does not return normally.
//
// SECURITY NOTE: This handler triggers binary replacement and process restart.
// It is registered under the protected API group; once JWT auth is enabled it
// will be automatically covered.  Until then, the default host binding of
// 127.0.0.1 limits exposure.
func Apply(ctx context.Context, result *CheckResult, mirror string, progress ProgressFunc, onRestarting func()) error {
	if result == nil || !result.HasUpdate {
		return fmt.Errorf("no update available")
	}

	downloadURL := ResolveDownloadURL(result.AssetURL, mirror)
	log.Printf("update: downloading %s from %s", result.AssetName, downloadURL)

	// ------------------------------------------------------------------
	// 1. Download the binary asset, computing SHA-256 as we stream.
	// ------------------------------------------------------------------
	progress(0, fmt.Sprintf("Downloading %s …", result.AssetName))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	client := newHTTPClient()
	// Increase timeout for large binary downloads.
	client.Timeout = 10 * time.Minute

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	totalBytes := resp.ContentLength // may be -1 if unknown
	hasher := sha256.New()
	var buf bytes.Buffer
	reader := io.TeeReader(resp.Body, hasher)

	if totalBytes > 0 {
		// Wrap with a progress-reporting reader.
		reader = io.TeeReader(
			&progressReader{r: resp.Body, total: totalBytes, progress: progress},
			hasher,
		)
	}

	if _, err := io.Copy(&buf, reader); err != nil {
		return fmt.Errorf("download body: %w", err)
	}
	digest := fmt.Sprintf("%x", hasher.Sum(nil))
	progress(100, "Download complete, verifying…")
	log.Printf("update: downloaded %d bytes, sha256=%s", buf.Len(), digest)

	// ------------------------------------------------------------------
	// 2. Best-effort checksum verification (TD7).
	// If a checksums file is present in the release, verify; skip otherwise.
	// ------------------------------------------------------------------
	if err := verifyChecksum(ctx, result, digest); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	// ------------------------------------------------------------------
	// 3. Atomically replace the running binary via minio/selfupdate.
	// ------------------------------------------------------------------
	progress(-1, "Applying update…")
	if err := selfupdate.Apply(bytes.NewReader(buf.Bytes()), selfupdate.Options{}); err != nil {
		// selfupdate rolls back automatically on failure.
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("apply failed and rollback also failed: apply=%v rollback=%v", err, rerr)
		}
		return fmt.Errorf("apply failed (binary rolled back): %w", err)
	}

	log.Printf("update: binary replaced successfully, restarting…")
	progress(-1, "Update applied. Restarting process…")

	// ------------------------------------------------------------------
	// 4. Re-exec: launch the new binary with the same args/env, then exit.
	// ------------------------------------------------------------------
	if err := restart(); err != nil {
		// The new binary is already on disk; just log and exit so the OS or
		// supervisor can restart the process.
		log.Printf("update: restart failed (%v), exiting so supervisor can restart", err)
	}

	// Notify the caller so it can broadcast the "restarting" SSE event to all
	// connected clients before the HTTP server dies.
	if onRestarting != nil {
		onRestarting()
	}

	// Give the SSE connection time to flush the "restarting" event to clients
	// before the process exits.
	time.Sleep(1500 * time.Millisecond)
	os.Exit(0)
	return nil // unreachable
}

// verifyChecksum tries to find a checksums file in the same release as result
// and verifies digest against it.  If no checksums asset is found, verification
// is skipped (best-effort per TD7).
func verifyChecksum(ctx context.Context, result *CheckResult, digest string) error {
	// We don't have a direct handle to the ReleaseInfo here, so we re-check the
	// cached result's asset list indirectly by fetching a well-known checksums
	// filename from the same release download URL path.
	//
	// Derive the checksums URL from the asset URL:
	//   .../releases/download/v1.0/psm_windows_amd64.exe
	// → .../releases/download/v1.0/checksums.txt
	checksumsURL := replaceAssetFilename(result.AssetURL, "checksums.txt")
	if checksumsURL == "" {
		log.Printf("update: cannot derive checksums URL, skipping verification")
		return nil
	}

	client := newHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumsURL, nil)
	if err != nil {
		log.Printf("update: build checksums request failed: %v — skipping verification", err)
		return nil
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("update: checksums fetch failed: %v — skipping verification", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Printf("update: no checksums.txt in release — skipping verification")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("update: checksums.txt returned HTTP %d — skipping verification", resp.StatusCode)
		return nil
	}

	// Parse "sha256  filename" lines.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		hash, filename := parts[0], parts[1]
		if filename == result.AssetName {
			if !strings.EqualFold(hash, digest) {
				return fmt.Errorf("sha256 mismatch: expected %s got %s", hash, digest)
			}
			log.Printf("update: checksum verified OK")
			return nil
		}
	}
	log.Printf("update: asset %q not found in checksums.txt — skipping verification", result.AssetName)
	return nil
}

// replaceAssetFilename swaps the last path segment of assetURL with newName.
// Returns "" if the URL cannot be parsed.
func replaceAssetFilename(assetURL, newName string) string {
	idx := strings.LastIndex(assetURL, "/")
	if idx < 0 {
		return ""
	}
	return assetURL[:idx+1] + newName
}

// progressReader wraps an io.Reader and calls progress with download percentage.
type progressReader struct {
	r        io.Reader
	total    int64
	read     int64
	last     int
	progress ProgressFunc
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.total > 0 {
		pct := int(p.read * 100 / p.total)
		if pct > p.last {
			p.last = pct
			p.progress(pct, fmt.Sprintf("Downloading… %d%%", pct))
		}
	}
	return n, err
}
