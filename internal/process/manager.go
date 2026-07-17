package process

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palconfig"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palmod"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/settings"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/steamcmd"
	"gorm.io/gorm"
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
	db              *gorm.DB
	streams         *logger.StreamManager
	logDir          string
	steamcmdPath    string
	steamUsername   string
	shutdownTimeout time.Duration

	mu           sync.Mutex
	running      map[int64]*procHandle // serverID -> running process
	installing   map[int64]struct{}    // serverID -> currently installing
	updatingMods map[int64]struct{}    // serverID -> currently updating mods

	// iniMu serializes PalModSettings.ini writes. UpdateMods (background
	// goroutine, final write) and RewriteModSettings (toggle/delete, HTTP
	// goroutine) both rebuild the file with a non-atomic os.WriteFile; without
	// this lock a toggle landing mid-update could observe/produce a torn file.
	// Writes are sub-millisecond and rare, so a single lock across servers is
	// sufficient and avoids per-server bookkeeping.
	iniMu sync.Mutex
}

// NewManager creates a process manager backed by the given database and stream
// manager. logDir is the base directory for per-server log files; steamcmdPath
// is the SteamCMD installation used to install server files; steamUsername is
// the Steam account used to download Workshop mods (empty → anonymous, which
// cannot download Palworld's paid workshop content).
func NewManager(db *gorm.DB, streams *logger.StreamManager, logDir, steamcmdPath, steamUsername string) *Manager {
	return &Manager{
		db:              db,
		streams:         streams,
		logDir:          logDir,
		steamcmdPath:    steamcmdPath,
		steamUsername:   steamUsername,
		shutdownTimeout: defaultShutdownTimeout,
		running:         make(map[int64]*procHandle),
		installing:      make(map[int64]struct{}),
		updatingMods:    make(map[int64]struct{}),
	}
}

// serverRow holds the fields needed to launch a server.
type serverRow struct {
	installPath string
	pid         int
	launchArgs  string
}

func (m *Manager) loadServer(serverID int64) (*serverRow, error) {
	var srv models.Server
	err := m.db.Select("install_path", "pid", "launch_args").First(&srv, serverID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("server %d not found", serverID)
		}
		return nil, err
	}
	return &serverRow{
		installPath: srv.InstallPath,
		pid:         srv.PID,
		launchArgs:  srv.LaunchArgs,
	}, nil
}

// setPID persists the process id fact for a server (0 means no process).
// A map update ensures pid=0 is written (a struct update would skip the zero
// value); GORM auto-refreshes updated_at.
func (m *Manager) setPID(serverID int64, pid int) error {
	return m.db.Model(&models.Server{}).Where("id = ?", serverID).
		Update("pid", pid).Error
}

// setError persists the last-error fact for a server. Update (single column)
// writes the empty string too, so clearError works via this path.
func (m *Manager) setError(serverID int64, msg string) error {
	return m.db.Model(&models.Server{}).Where("id = ?", serverID).
		Update("last_error", msg).Error
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

	// Resolve the executable to spawn and its working directory. On Windows this
	// is the console server binary run directly (not the PalServer.exe launcher)
	// so its stdout/stderr can be captured; on Unix it is PalServer.sh.
	exe, workDir, err := launchTarget(srv.installPath)
	if err != nil {
		return err
	}

	// Build launch arguments from the persisted configuration, then append the
	// platform's log-forcing flags so the server actually emits its runtime log
	// to the stdout/stderr we capture (see logArgs).
	launchArgs, err := palconfig.ParseLaunchArgs(srv.launchArgs)
	if err != nil {
		return fmt.Errorf("server %d: %w", serverID, err)
	}
	args := launchArgs.ToArgs()
	// args := append(launchArgs.ToArgs(), logArgs()...)

	// Compose log sinks: persist to disk and broadcast live lines to SSE clients.
	// KindServer keeps the running server's output separate from SteamCMD logs.
	capture := logger.NewCapture(serverID, logger.KindServer, m.logDir)
	broadcaster := logger.NewBroadcastWriter(m.streams, serverID, logger.KindServer)
	out := io.MultiWriter(capture, broadcaster)

	// Take over the server process's own stdout/stderr and funnel both into the
	// server-kind pipeline (persist to disk + live SSE). os/exec copies the
	// child's output through goroutines that cmd.Wait blocks on, so every line is
	// flushed before monitor closes the capture. Using the same writer for both
	// streams lets os/exec share a single pipe, so concurrent stdout/stderr
	// writes do not interleave mid-line.
	cmd := exec.Command(exe, args...)
	cmd.Dir = workDir
	cmd.SysProcAttr = sysProcAttr()
	cmd.Stdout = out
	cmd.Stderr = out

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

	// monitor waits for the process to exit, then closes the capture and clears
	// the recorded pid.
	go m.monitor(serverID, handle, capture)
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

	var srv models.Server
	if err := m.db.Select("install_path").First(&srv, serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("server %d not found", serverID)
		}
		return err
	}
	installPath := srv.InstallPath

	// SteamCMD runs without holding m.mu so status queries stay responsive.
	err := steamcmd.InstallPalworldServer(installPath, m.steamcmdPath, out)
	if err != nil {
		_ = m.setError(serverID, err.Error())
		_ = m.db.Model(&models.Server{}).Where("id = ?", serverID).Update("installed", false).Error
		return err
	}

	_ = m.clearError(serverID)
	_ = m.db.Model(&models.Server{}).Where("id = ?", serverID).Update("installed", true).Error
	return nil
}

// resolveSteamUsername returns the Steam account to download mods with. The DB
// setting (steam_username, adjustable at runtime via the app-in login flow)
// takes precedence; the config value (m.steamUsername) is only a fallback
// default. Empty → anonymous, which cannot download Palworld's paid workshop
// content. A DB read error falls back to the config value rather than failing
// the update.
func (m *Manager) resolveSteamUsername() string {
	if v, err := settings.Get(m.db, settings.KeySteamUsername); err == nil {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return m.steamUsername
}

// IsUpdatingMods reports whether a mod update is currently in progress.
func (m *Manager) IsUpdatingMods(serverID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.updatingMods[serverID]
	return ok
}

// UpdateMods downloads every mod registered for the server via SteamCMD,
// deploys each into <installPath>/Mods/Workshop/<workshopID>/, backfills
// package_name/version/install_path from each mod's Info.json, then rewrites
// PalModSettings.ini so only enabled mods appear in ActiveModList. SteamCMD
// output is written to out (persisted + streamed by the caller, mirroring
// InstallServer).
//
// Concurrency: it is refused while an install or another mod update is in
// progress, but NOT while the server is running — copying into Mods/Workshop
// does not touch the live process; the change only takes effect on the next
// restart (the UI surfaces the "needs restart" hint). Progress is tracked in
// memory only (never persisted), like InstallServer.
//
// A single mod's failure (invalid id, anonymous login denied, missing
// Info.json) is recorded and processing continues with the rest (partial
// success). The aggregate outcome is persisted to last_error: cleared on full
// success, populated with a readable multi-line summary otherwise.
func (m *Manager) UpdateMods(serverID int64, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}

	m.mu.Lock()
	if _, ok := m.installing[serverID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("server %d is installing", serverID)
	}
	if _, ok := m.updatingMods[serverID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("server %d mods are already updating", serverID)
	}
	m.updatingMods[serverID] = struct{}{}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.updatingMods, serverID)
		m.mu.Unlock()
	}()

	var srv models.Server
	if err := m.db.Select("install_path").First(&srv, serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("server %d not found", serverID)
		}
		return err
	}
	installPath := srv.InstallPath
	if installPath == "" {
		err := fmt.Errorf("server %d has no install directory", serverID)
		_ = m.setError(serverID, err.Error())
		return err
	}

	var mods []models.Mod
	if err := m.db.Where("server_id = ?", serverID).Find(&mods).Error; err != nil {
		return err
	}

	// Resolve the download account once (DB setting > config fallback).
	steamUsername := m.resolveSteamUsername()

	var failures []string
	for i := range mods {
		mod := &mods[i]
		fmt.Fprintf(out, "==> Processing mod %s (%s)...\n", mod.WorkshopID, mod.Name)

		downloaded, err := steamcmd.DownloadWorkshopItem(m.steamcmdPath, steamUsername, mod.WorkshopID, out)
		if err != nil {
			failures = append(failures, fmt.Sprintf("mod %s: %v", mod.WorkshopID, err))
			continue
		}

		dst, err := palmod.Deploy(installPath, mod.WorkshopID, downloaded)
		if err != nil {
			failures = append(failures, fmt.Sprintf("mod %s: deploy: %v", mod.WorkshopID, err))
			continue
		}

		info, err := palmod.ParseInfo(dst)
		if err != nil {
			failures = append(failures, fmt.Sprintf("mod %s: %v", mod.WorkshopID, err))
			// Still record the install path so the content is tracked.
			_ = m.db.Model(&models.Mod{}).Where("id = ?", mod.ID).
				Update("install_path", dst).Error
			mod.InstallPath = dst
			continue
		}

		if !info.IsServer {
			fmt.Fprintf(out, "==> Warning: mod %s (%s) is not marked IsServer; it may not be designed for dedicated servers\n",
				mod.WorkshopID, info.PackageName)
		}

		// Select forces all listed columns to be written even when zero-valued
		// (empty PackageName/ModName/Version, nil Tags) and — unlike a map update —
		// routes the Tags []string through the column's json serializer instead of
		// emitting a raw SQL tuple.
		if err := m.db.Model(&models.Mod{}).Where("id = ?", mod.ID).
			Select("package_name", "mod_name", "version", "tags", "install_path").
			Updates(models.Mod{
				PackageName: info.PackageName,
				ModName:     info.ModName,
				Version:     info.Version,
				Tags:        info.Tags,
				InstallPath: dst,
			}).Error; err != nil {
			failures = append(failures, fmt.Sprintf("mod %s: persist metadata: %v", mod.WorkshopID, err))
			continue
		}
		// Reflect the backfill locally so the ActiveModList uses the resolved name.
		mod.PackageName = info.PackageName
		mod.ModName = info.ModName
		mod.Version = info.Version
		mod.Tags = info.Tags
		mod.InstallPath = dst
	}

	// Rewrite PalModSettings.ini from the (possibly partially updated) enabled
	// set. Only mods with a resolved PackageName can be referenced.
	var enabled []palmod.EnabledMod
	for i := range mods {
		if mods[i].Enabled && strings.TrimSpace(mods[i].PackageName) != "" {
			enabled = append(enabled, palmod.EnabledMod{PackageName: mods[i].PackageName})
		}
	}
	m.iniMu.Lock()
	err := palmod.WriteModSettings(installPath, enabled)
	m.iniMu.Unlock()
	if err != nil {
		failures = append(failures, fmt.Sprintf("write PalModSettings.ini: %v", err))
	}

	if len(failures) > 0 {
		msg := "Mod update completed with errors:\n" + strings.Join(failures, "\n")
		fmt.Fprintf(out, "==> %s\n", msg)
		_ = m.setError(serverID, msg)
		return errors.New(msg)
	}

	fmt.Fprintf(out, "==> Mod update complete\n")
	_ = m.clearError(serverID)
	return nil
}

// RewriteModSettings recomputes PalModSettings.ini for a server from its current
// mod rows (used after a toggle or delete, which do not re-download). Only
// enabled mods with a resolved PackageName are written to ActiveModList.
func (m *Manager) RewriteModSettings(serverID int64) error {
	var srv models.Server
	if err := m.db.Select("install_path").First(&srv, serverID).Error; err != nil {
		return err
	}
	if srv.InstallPath == "" {
		return fmt.Errorf("server %d has no install directory", serverID)
	}

	var mods []models.Mod
	if err := m.db.Where("server_id = ?", serverID).Find(&mods).Error; err != nil {
		return err
	}

	var enabled []palmod.EnabledMod
	for i := range mods {
		if mods[i].Enabled && strings.TrimSpace(mods[i].PackageName) != "" {
			enabled = append(enabled, palmod.EnabledMod{PackageName: mods[i].PackageName})
		}
	}
	m.iniMu.Lock()
	defer m.iniMu.Unlock()
	return palmod.WriteModSettings(srv.InstallPath, enabled)
}
