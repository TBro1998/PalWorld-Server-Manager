package process

import (
	"fmt"
	"os/exec"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
)

// monitor waits for a server process to exit, then updates the database and
// clears the running map. It also closes the log capture. Runs in its own
// goroutine, one per started server.
func (m *Manager) monitor(serverID int64, cmd *exec.Cmd, capture *logger.Capture) {
	err := cmd.Wait()

	m.mu.Lock()
	delete(m.running, serverID)
	m.mu.Unlock()

	if capture != nil {
		capture.Close()
	}

	if err != nil {
		// Non-zero exit or signal-terminated (expected on graceful stop).
		fmt.Printf("server %d process exited: %v\n", serverID, err)
	}

	if updErr := m.updateStatus(serverID, StatusStopped, 0); updErr != nil {
		fmt.Printf("warning: failed to mark server %d stopped: %v\n", serverID, updErr)
	}
}

// ReconcileOnStartup corrects stale state left over from a previous run of the
// application. Any server marked running whose recorded PID is no longer alive
// is reset to stopped. Additionally, any server still marked installing is an
// orphan: the SteamCMD subprocess died with the previous application process
// and cannot survive a restart, so it is reset to stopped unconditionally.
// Should be called once during startup.
func (m *Manager) ReconcileOnStartup() error {
	rows, err := m.db.Query(`SELECT id, pid FROM servers WHERE status = ?`, StatusRunning)
	if err != nil {
		return err
	}
	defer rows.Close()

	type stale struct {
		id  int64
		pid int
	}
	var toReset []stale

	for rows.Next() {
		var id int64
		var pid int
		if err := rows.Scan(&id, &pid); err != nil {
			return err
		}
		if pid <= 0 || !isProcessAlive(pid) {
			toReset = append(toReset, stale{id: id, pid: pid})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, s := range toReset {
		if err := m.updateStatus(s.id, StatusStopped, 0); err != nil {
			return err
		}
		fmt.Printf("reconciled server %d: marked stopped (pid %d not alive)\n", s.id, s.pid)
	}

	// Installs never survive an application restart: the SteamCMD subprocess died
	// with the previous process, so any server still marked "installing" is
	// orphaned. Reset it to stopped unconditionally so the user can retry.
	// ReconcileInstalled (called next in server startup) then recomputes the
	// `installed` flag from on-disk reality, so a partial install shows as
	// "not installed" rather than stuck.
	if _, err := m.db.Exec(
		`UPDATE servers SET status = ?, pid = 0, updated_at = CURRENT_TIMESTAMP WHERE status = ?`,
		StatusStopped, StatusInstalling); err != nil {
		return err
	}
	return nil
}

// ReconcileInstalled refreshes the persisted `installed` flag for every server
// by checking whether the launcher executable exists at its install path.
// Called once at startup so the flag reflects on-disk reality without probing
// on every API request.
func (m *Manager) ReconcileInstalled() error {
	rows, err := m.db.Query(`SELECT id, install_path FROM servers`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type entry struct {
		id          int64
		installPath string
	}
	var servers []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.installPath); err != nil {
			return err
		}
		servers = append(servers, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, s := range servers {
		installed := IsInstalled(s.installPath)
		if _, err := m.db.Exec(
			`UPDATE servers SET installed = ? WHERE id = ?`, installed, s.id); err != nil {
			return err
		}
	}
	return nil
}
