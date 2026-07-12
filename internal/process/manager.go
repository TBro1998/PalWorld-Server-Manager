package process

import (
	"database/sql"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
)

// Server status values persisted in the servers table.
const (
	StatusStopped    = "stopped"
	StatusRunning    = "running"
	StatusInstalling = "installing"
	StatusError      = "error"
)

// defaultShutdownTimeout is how long to wait for a graceful stop before force-kill.
const defaultShutdownTimeout = 10 * time.Second

// Manager owns the lifecycle of running Palworld server processes.
type Manager struct {
	db              *sql.DB
	streams         *logger.StreamManager
	logDir          string
	shutdownTimeout time.Duration

	mu      sync.Mutex
	running map[int64]*exec.Cmd // serverID -> running process
}

// NewManager creates a process manager backed by the given database and stream
// manager. logDir is the base directory for per-server log files.
func NewManager(db *sql.DB, streams *logger.StreamManager, logDir string) *Manager {
	return &Manager{
		db:              db,
		streams:         streams,
		logDir:          logDir,
		shutdownTimeout: defaultShutdownTimeout,
		running:         make(map[int64]*exec.Cmd),
	}
}

// serverRow holds the fields needed to launch a server.
type serverRow struct {
	installPath string
	status      string
	pid         int
}

func (m *Manager) loadServer(serverID int64) (*serverRow, error) {
	row := m.db.QueryRow(
		`SELECT install_path, status, pid FROM servers WHERE id = ?`, serverID)
	var s serverRow
	if err := row.Scan(&s.installPath, &s.status, &s.pid); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("server %d not found", serverID)
		}
		return nil, err
	}
	return &s, nil
}

func (m *Manager) updateStatus(serverID int64, status string, pid int) error {
	_, err := m.db.Exec(
		`UPDATE servers SET status = ?, pid = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, pid, serverID)
	return err
}

// StartServer launches the Palworld dedicated server for the given ID. It fails
// if the server is already running or not in a stoppable state.
func (m *Manager) StartServer(serverID int64) error {
	m.mu.Lock()
	if _, ok := m.running[serverID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("server %d is already running", serverID)
	}
	m.mu.Unlock()

	srv, err := m.loadServer(serverID)
	if err != nil {
		return err
	}
	if srv.status == StatusInstalling {
		return fmt.Errorf("server %d is currently installing", serverID)
	}

	exe, err := serverExecutable(srv.installPath)
	if err != nil {
		return err
	}

	// Compose log sinks: persist to disk and broadcast live lines to SSE clients.
	capture := logger.NewCapture(serverID, m.logDir)
	broadcaster := logger.NewBroadcastWriter(m.streams, serverID)
	out := io.MultiWriter(capture, broadcaster)

	cmd := exec.Command(exe)
	cmd.Dir = srv.installPath
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = sysProcAttr()

	if err := cmd.Start(); err != nil {
		capture.Close()
		_ = m.updateStatus(serverID, StatusError, 0)
		return fmt.Errorf("failed to start server: %w", err)
	}

	pid := cmd.Process.Pid
	m.mu.Lock()
	m.running[serverID] = cmd
	m.mu.Unlock()

	if err := m.updateStatus(serverID, StatusRunning, pid); err != nil {
		// Process started but DB update failed; record and continue monitoring.
		fmt.Printf("warning: failed to persist running status for server %d: %v\n", serverID, err)
	}

	go m.monitor(serverID, cmd, capture)
	return nil
}

// StopServer gracefully stops a running server, escalating to a forced kill on
// timeout. It is a no-op (returns nil) if the server is not running.
func (m *Manager) StopServer(serverID int64) error {
	m.mu.Lock()
	cmd, ok := m.running[serverID]
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
		return m.updateStatus(serverID, StatusStopped, 0)
	}

	pid := cmd.Process.Pid
	if err := killProcess(pid, m.shutdownTimeout); err != nil {
		return fmt.Errorf("failed to stop server %d: %w", serverID, err)
	}
	// monitor() will observe the exit and update the database + running map.
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
