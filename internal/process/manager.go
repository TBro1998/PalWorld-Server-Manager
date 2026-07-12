package process

import (
	"database/sql"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palconfig"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/steamcmd"
)

// Derived status values. These are NOT persisted; DeriveStatus computes them
// from in-memory state plus persisted facts (pid, last_error).
const (
	StatusStopped    = "stopped"
	StatusRunning    = "running"
	StatusInstalling = "installing"
	StatusError      = "error"
)

// defaultShutdownTimeout is how long to wait for a graceful stop before force-kill.
const defaultShutdownTimeout = 10 * time.Second

// procHandle tracks a server process the manager knows about. cmd is nil when
// the process was adopted by PID after a manager restart (a running orphan that
// cannot be Wait()ed on and is instead polled for liveness).
type procHandle struct {
	cmd *exec.Cmd
	pid int
}

// Manager owns the lifecycle of running Palworld server processes.
type Manager struct {
	db              *sql.DB
	streams         *logger.StreamManager
	logDir          string
	steamcmdPath    string
	shutdownTimeout time.Duration

	mu         sync.Mutex
	running    map[int64]*procHandle // serverID -> running process
	installing map[int64]struct{}    // serverID -> currently installing
}

// NewManager creates a process manager backed by the given database and stream
// manager. logDir is the base directory for per-server log files; steamcmdPath
// is the SteamCMD installation used to install server files.
func NewManager(db *sql.DB, streams *logger.StreamManager, logDir, steamcmdPath string) *Manager {
	return &Manager{
		db:              db,
		streams:         streams,
		logDir:          logDir,
		steamcmdPath:    steamcmdPath,
		shutdownTimeout: defaultShutdownTimeout,
		running:         make(map[int64]*procHandle),
		installing:      make(map[int64]struct{}),
	}
}

// serverRow holds the fields needed to launch a server.
type serverRow struct {
	installPath string
	pid         int
	launchArgs  string
}

func (m *Manager) loadServer(serverID int64) (*serverRow, error) {
	row := m.db.QueryRow(
		`SELECT install_path, pid, launch_args FROM servers WHERE id = ?`, serverID)
	var s serverRow
	if err := row.Scan(&s.installPath, &s.pid, &s.launchArgs); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("server %d not found", serverID)
		}
		return nil, err
	}
	return &s, nil
}

// setPID persists the process id fact for a server (0 means no process).
func (m *Manager) setPID(serverID int64, pid int) error {
	_, err := m.db.Exec(
		`UPDATE servers SET pid = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		pid, serverID)
	return err
}

// setError persists the last-error fact for a server.
func (m *Manager) setError(serverID int64, msg string) error {
	_, err := m.db.Exec(
		`UPDATE servers SET last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		msg, serverID)
	return err
}

// clearError clears the last-error fact for a server.
func (m *Manager) clearError(serverID int64) error {
	return m.setError(serverID, "")
}

// DeriveStatus computes the reported status from in-memory state and the given
// persisted last_error, following the precedence: installing > running > error
// > stopped.
func (m *Manager) DeriveStatus(serverID int64, lastError string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.installing[serverID]; ok {
		return StatusInstalling
	}
	if _, ok := m.running[serverID]; ok {
		return StatusRunning
	}
	if lastError != "" {
		return StatusError
	}
	return StatusStopped
}

// IsInstalling reports whether an installation is currently in progress.
func (m *Manager) IsInstalling(serverID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.installing[serverID]
	return ok
}

// StartServer launches the Palworld dedicated server for the given ID. It fails
// if the server is already running or currently installing.
func (m *Manager) StartServer(serverID int64) error {
	if m.IsRunning(serverID) {
		return fmt.Errorf("server %d is already running", serverID)
	}
	if m.IsInstalling(serverID) {
		return fmt.Errorf("server %d is currently installing", serverID)
	}

	srv, err := m.loadServer(serverID)
	if err != nil {
		return err
	}

	exe, err := serverExecutable(srv.installPath)
	if err != nil {
		return err
	}

	// Build launch arguments from the persisted configuration.
	launchArgs, err := palconfig.ParseLaunchArgs(srv.launchArgs)
	if err != nil {
		return fmt.Errorf("server %d: %w", serverID, err)
	}
	args := launchArgs.ToArgs()

	// Compose log sinks: persist to disk and broadcast live lines to SSE clients.
	capture := logger.NewCapture(serverID, m.logDir)
	broadcaster := logger.NewBroadcastWriter(m.streams, serverID)
	out := io.MultiWriter(capture, broadcaster)

	cmd := exec.Command(exe, args...)
	cmd.Dir = srv.installPath
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = sysProcAttr()

	if err := cmd.Start(); err != nil {
		capture.Close()
		_ = m.setError(serverID, err.Error())
		return fmt.Errorf("failed to start server: %w", err)
	}

	pid := cmd.Process.Pid
	handle := &procHandle{cmd: cmd, pid: pid}
	m.mu.Lock()
	m.running[serverID] = handle
	m.mu.Unlock()

	if err := m.setPID(serverID, pid); err != nil {
		// Process started but DB update failed; record and continue monitoring.
		fmt.Printf("warning: failed to persist pid for server %d: %v\n", serverID, err)
	}
	_ = m.clearError(serverID)

	go m.monitor(serverID, &procHandle{cmd: cmd, pid: pid}, capture)
	return nil
}

// StopServer gracefully stops a running server, escalating to a forced kill on
// timeout. It is a no-op (returns nil) if the server is not running.
func (m *Manager) StopServer(serverID int64) error {
	m.mu.Lock()
	handle, ok := m.running[serverID]
	m.mu.Unlock()

	if !ok {
		// Not tracked in memory; fall back to PID recorded in the database.
		srv, err := m.loadServer(serverID)
		if err != nil {
			return err
		}
		if srv.pid > 0 && isProcessAlive(srv.pid) {
			if err := killProcess(srv.pid, m.shutdownTimeout); err != nil {
				return err
			}
		}
		if err := m.setPID(serverID, 0); err != nil {
			return err
		}
		return m.clearError(serverID)
	}

	if err := killProcess(handle.pid, m.shutdownTimeout); err != nil {
		return fmt.Errorf("failed to stop server %d: %w", serverID, err)
	}
	// monitor() will observe the exit and clear pid + running map.
	_ = m.clearError(serverID)
	return nil
}

// RestartServer stops (if running) and then starts the server.
func (m *Manager) RestartServer(serverID int64) error {
	if err := m.StopServer(serverID); err != nil {
		return err
	}
	// Wait for the process to fully exit and monitor to clear the running map.
	deadline := time.Now().Add(m.shutdownTimeout + 5*time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		_, stillRunning := m.running[serverID]
		m.mu.Unlock()
		if !stillRunning {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return m.StartServer(serverID)
}

// IsRunning reports whether the manager is currently tracking a live process
// for the server.
func (m *Manager) IsRunning(serverID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.running[serverID]
	return ok
}

// InstallServer installs Palworld server files via SteamCMD, writing SteamCMD
// output to out. Installation progress is tracked in memory only (the installing
// set), never persisted, so a manager restart mid-install cannot leave a server
// stuck. On failure the error is recorded in last_error and installed is set to
// false; on success last_error is cleared and installed is set to true.
func (m *Manager) InstallServer(serverID int64, out io.Writer) error {
	m.mu.Lock()
	if _, ok := m.running[serverID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("server %d is running", serverID)
	}
	if _, ok := m.installing[serverID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("server %d is already installing", serverID)
	}
	m.installing[serverID] = struct{}{}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.installing, serverID)
		m.mu.Unlock()
	}()

	var installPath string
	if err := m.db.QueryRow(
		`SELECT install_path FROM servers WHERE id = ?`, serverID,
	).Scan(&installPath); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("server %d not found", serverID)
		}
		return err
	}

	// SteamCMD runs without holding m.mu so status queries stay responsive.
	err := steamcmd.InstallPalworldServer(installPath, m.steamcmdPath, out)
	if err != nil {
		_ = m.setError(serverID, err.Error())
		_, _ = m.db.Exec(`UPDATE servers SET installed = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, serverID)
		return err
	}

	_ = m.clearError(serverID)
	_, _ = m.db.Exec(`UPDATE servers SET installed = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, serverID)
	return nil
}
