package logger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	// maxLogSize is the maximum size of a single log file before rotation (10MB).
	maxLogSize = 10 * 1024 * 1024
	// maxLogFiles is the number of rotated log files to retain per server.
	maxLogFiles = 10
)

// Log kinds partition a server's output into independent streams so the game
// server's runtime logs and SteamCMD's install/update logs never mix. Each kind
// has its own on-disk directory and its own live SSE channel.
const (
	// KindServer is the running Palworld server's game log (tailed from the
	// Unreal Engine log file, since the Windows launcher does not expose the
	// real server's stdout).
	KindServer = "server"
	// KindSteamCMD is SteamCMD's install/update output for a server.
	KindSteamCMD = "steamcmd"
)

// Capture implements io.Writer, persisting a server's log output to a rotating
// log file. It is safe for concurrent use.
type Capture struct {
	serverID int64
	dir      string

	mu      sync.Mutex
	file    *os.File
	written int64
}

// NewCapture creates a Capture for the given server and log kind, writing under
// <logDir>/server_<id>/<kind>/current.log. The directory is created on first
// write.
func NewCapture(serverID int64, kind, logDir string) *Capture {
	return &Capture{
		serverID: serverID,
		dir:      serverLogDir(logDir, serverID, kind),
	}
}

func serverLogDir(logDir string, serverID int64, kind string) string {
	return filepath.Join(logDir, fmt.Sprintf("server_%d", serverID), kind)
}

func (c *Capture) currentPath() string {
	return filepath.Join(c.dir, "current.log")
}

// Write appends p to the current log file, rotating first if the size limit
// would be exceeded. Implements io.Writer.
func (c *Capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.file == nil {
		if err := c.openLocked(); err != nil {
			return 0, err
		}
	}

	if c.written+int64(len(p)) > maxLogSize {
		if err := c.rotateLocked(); err != nil {
			return 0, err
		}
	}

	n, err := c.file.Write(p)
	c.written += int64(n)
	return n, err
}

// openLocked opens (or creates) the current log file. Caller must hold c.mu.
func (c *Capture) openLocked() error {
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	f, err := os.OpenFile(c.currentPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	c.file = f
	c.written = info.Size()
	return nil
}

// rotateLocked closes the current file, renames it with a timestamp, opens a
// fresh current.log, and prunes old files. Caller must hold c.mu.
func (c *Capture) rotateLocked() error {
	if c.file != nil {
		c.file.Close()
		c.file = nil
	}
	// time.Now is acceptable here (runtime rotation, not part of any replayable flow).
	stamp := time.Now().Format("2006-01-02_150405")
	rotated := filepath.Join(c.dir, fmt.Sprintf("%s.log", stamp))
	// Ignore rename error if current.log doesn't exist yet.
	_ = os.Rename(c.currentPath(), rotated)

	c.pruneLocked()

	return c.openLocked()
}

// pruneLocked removes the oldest rotated logs beyond maxLogFiles.
func (c *Capture) pruneLocked() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	var rotated []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "current.log" {
			continue
		}
		rotated = append(rotated, e.Name())
	}
	if len(rotated) <= maxLogFiles {
		return
	}
	sort.Strings(rotated) // timestamped names sort chronologically
	for _, name := range rotated[:len(rotated)-maxLogFiles] {
		_ = os.Remove(filepath.Join(c.dir, name))
	}
}

// Close flushes and closes the underlying log file.
func (c *Capture) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.file != nil {
		err := c.file.Close()
		c.file = nil
		return err
	}
	return nil
}

// ReadLogs returns the last `lines` lines from a server's current log file for
// the given log kind. If lines <= 0, a default of 200 is used.
func ReadLogs(logDir string, serverID int64, kind string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 200
	}
	path := filepath.Join(serverLogDir(logDir, serverID, kind), "current.log")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	// Ring buffer of the last N lines.
	buf := make([]string, 0, lines)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if len(buf) == lines {
			buf = buf[1:]
		}
		buf = append(buf, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return buf, nil
}
