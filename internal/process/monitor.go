package process

import (
	"fmt"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
)

// pollInterval is how often adopted (cmd==nil) processes are polled for exit.
const pollInterval = 3 * time.Second

// monitor waits for a server process to exit, then clears the recorded pid and
// removes the server from the running map. It also stops the log tailer and
// closes the log capture. Runs in its own goroutine, one per started (or
// adopted) server. When h.cmd is nil the process was adopted by PID and cannot
// be Wait()ed on, so its liveness is polled instead.
//
// capture, stopTail and tailDone are nil for adopted processes (no tailer was
// started for them). When present, the tailer is stopped and awaited before the
// capture is closed so no tail write races with Close.
func (m *Manager) monitor(serverID int64, h *procHandle, capture *logger.Capture, stopTail chan struct{}, tailDone <-chan struct{}) {
	if h.cmd != nil {
		if err := h.cmd.Wait(); err != nil {
			// Non-zero exit or signal-terminated (expected on graceful stop).
			fmt.Printf("server %d process exited: %v\n", serverID, err)
		}
	} else {
		for isProcessAlive(h.pid) {
			time.Sleep(pollInterval)
		}
	}

	m.mu.Lock()
	delete(m.running, serverID)
	m.mu.Unlock()

	if stopTail != nil {
		close(stopTail)
	}
	if tailDone != nil {
		<-tailDone
	}
	if capture != nil {
		capture.Close()
	}

	if err := m.setPID(serverID, 0); err != nil {
		fmt.Printf("warning: failed to clear pid for server %d: %v\n", serverID, err)
	}
}

// ReconcileOnStartup adopts server processes that survived a previous run of
// the application, using persisted facts only. Any server with a recorded PID
// that is still alive is re-registered as running with a polling monitor (an
// orphan cannot be Wait()ed on); a recorded PID that is no longer alive is
// cleared to 0. Installations are never reconciled: they are tracked purely in
// memory, so a restart mid-install leaves no stuck record — such a server has
// pid=0 and an empty last_error and therefore derives as "stopped", allowing a
// clean retry. Should be called once during startup.
func (m *Manager) ReconcileOnStartup() error {
	var servers []models.Server
	if err := m.db.Select("id", "pid").Where("pid > 0").Find(&servers).Error; err != nil {
		return err
	}

	for _, s := range servers {
		if isProcessAlive(s.PID) {
			handle := &procHandle{cmd: nil, pid: s.PID}
			m.mu.Lock()
			m.running[s.ID] = handle
			m.mu.Unlock()
			go m.monitor(s.ID, &procHandle{cmd: nil, pid: s.PID}, nil, nil, nil)
			fmt.Printf("reconciled server %d: adopted running process (pid %d)\n", s.ID, s.PID)
		} else {
			if err := m.setPID(s.ID, 0); err != nil {
				return err
			}
			fmt.Printf("reconciled server %d: cleared stale pid %d (not alive)\n", s.ID, s.PID)
		}
	}
	return nil
}

// ReconcileInstalled refreshes the persisted `installed` flag for every server
// by checking whether the launcher executable exists at its install path.
// Called once at startup so the flag reflects on-disk reality without probing
// on every API request.
func (m *Manager) ReconcileInstalled() error {
	var servers []models.Server
	if err := m.db.Select("id", "install_path").Find(&servers).Error; err != nil {
		return err
	}

	for _, s := range servers {
		installed := IsInstalled(s.InstallPath)
		if err := m.db.Model(&models.Server{}).Where("id = ?", s.ID).
			Update("installed", installed).Error; err != nil {
			return err
		}
	}
	return nil
}
