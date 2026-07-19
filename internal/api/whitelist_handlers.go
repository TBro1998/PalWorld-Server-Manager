package api

import (
	"bufio"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// whitelistPath returns the conventional whitelist.txt path inside a server's
// install directory. Palworld reads this file when UseAuthPassword is enabled.
func whitelistPath(installPath string) string {
	return filepath.Join(installPath, "Pal", "Saved", "Config", "WindowsServer", "whitelist.txt")
}

// readWhitelist reads the whitelist file and returns non-empty, trimmed lines.
// Returns an empty (non-nil) slice when the file does not exist yet.
func readWhitelist(path string) ([]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			entries = append(entries, line)
		}
	}
	return entries, sc.Err()
}

// writeWhitelist writes entries to the whitelist file, one per line.
// Parent directories are created if they do not exist.
func writeWhitelist(path string, entries []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, e := range entries {
		if _, err := w.WriteString(e + "\n"); err != nil {
			return err
		}
	}
	return w.Flush()
}

// resolveWhitelistServer is a shared helper: parse :id, load install path, and
// return the path to that server's whitelist.txt. On failure it writes the
// appropriate HTTP error and returns ("", false).
func (r *Router) resolveWhitelistServer(c *gin.Context) (wlPath string, ok bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return "", false
	}
	installPath, _, _, _, err := r.loadServerPathState(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return "", false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return "", false
	}
	if installPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server install path is not set"})
		return "", false
	}
	return whitelistPath(installPath), true
}

// GetWhitelist returns the current list of whitelisted Steam UIDs for a server.
//
// GET /api/servers/:id/whitelist
// Response: { "entries": ["76561198...", ...] }
func (r *Router) GetWhitelist(c *gin.Context) {
	path, ok := r.resolveWhitelistServer(c)
	if !ok {
		return
	}
	entries, err := readWhitelist(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read whitelist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

type whitelistAddRequest struct {
	UID string `json:"uid" binding:"required"`
}

// AddWhitelistEntry appends a UID to the server's whitelist (deduplicates by
// case-insensitive comparison).
//
// POST /api/servers/:id/whitelist
// Body: { "uid": "76561198..." }
// Response: { "entries": [...updated list...] }
func (r *Router) AddWhitelistEntry(c *gin.Context) {
	path, ok := r.resolveWhitelistServer(c)
	if !ok {
		return
	}
	var req whitelistAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid := strings.TrimSpace(req.UID)
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid must not be empty"})
		return
	}

	entries, err := readWhitelist(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read whitelist"})
		return
	}
	for _, e := range entries {
		if strings.EqualFold(e, uid) {
			// Already present — return the current list unchanged.
			c.JSON(http.StatusOK, gin.H{"entries": entries})
			return
		}
	}
	entries = append(entries, uid)
	if err := writeWhitelist(path, entries); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write whitelist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

type whitelistRemoveRequest struct {
	UID string `json:"uid" binding:"required"`
}

// RemoveWhitelistEntry removes a UID from the server's whitelist (case-insensitive).
//
// DELETE /api/servers/:id/whitelist
// Body: { "uid": "76561198..." }
// Response: { "entries": [...updated list...] }
func (r *Router) RemoveWhitelistEntry(c *gin.Context) {
	path, ok := r.resolveWhitelistServer(c)
	if !ok {
		return
	}
	var req whitelistRemoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid := strings.TrimSpace(req.UID)

	entries, err := readWhitelist(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read whitelist"})
		return
	}
	filtered := make([]string, 0, len(entries))
	for _, e := range entries {
		if !strings.EqualFold(e, uid) {
			filtered = append(filtered, e)
		}
	}
	if err := writeWhitelist(path, filtered); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write whitelist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": filtered})
}
