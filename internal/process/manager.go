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
	deployingMods map[int64]struct{}   // serverID -> currently deploying mods to server dir
	downloadingGlobalMods map[string]struct{} // workshopID -> currently downloading from SteamCMD

	// iniMu serializes PalModSettings.ini writes. DeployServerMods (background
	// goroutine, final write) and RewriteModSettings (toggle/unlink, HTTP
	// goroutine) both rebuild the file with a non-atomic os.WriteFile; without
	// this lock a toggle landing mid-deploy could observe/produce a torn file.
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
		running:               make(map[int64]*procHandle),
		installing:            make(map[int64]struct{}),
		deployingMods:         make(map[int64]struct{}),
		downloadingGlobalMods: make(map[string]struct{}),
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

	// Auto-sync any mods whose global library version differs from what is
	// deployed to the server. Failures are warnings only — never block the start.
	m.syncModsOnStart(serverID, srv.installPath)

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

// IsDeployingMods reports whether a mod deployment is currently in progress for a server.
func (m *Manager) IsDeployingMods(serverID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.deployingMods[serverID]
	return ok
}

// IsDownloadingGlobalMod reports whether a global SteamCMD download is in
// progress for the given workshopID.
func (m *Manager) IsDownloadingGlobalMod(workshopID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.downloadingGlobalMods[workshopID]
	return ok
}

// DownloadGlobalMod downloads a single mod from Steam Workshop into the SteamCMD
// staging area and backfills the global mods table with metadata from Info.json.
// Progress is written to out. The mod must already exist in the global library.
func (m *Manager) DownloadGlobalMod(modID int64, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}

	var mod models.Mod
	if err := m.db.First(&mod, modID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("mod %d not found", modID)
		}
		return err
	}

	m.mu.Lock()
	if _, ok := m.downloadingGlobalMods[mod.WorkshopID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("mod %s is already downloading", mod.WorkshopID)
	}
	m.downloadingGlobalMods[mod.WorkshopID] = struct{}{}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.downloadingGlobalMods, mod.WorkshopID)
		m.mu.Unlock()
	}()

	steamUsername := m.resolveSteamUsername()
	fmt.Fprintf(out, "==> Downloading mod %s (%s)...\n", mod.WorkshopID, mod.Name)

	downloadPath, err := steamcmd.DownloadWorkshopItem(m.steamcmdPath, steamUsername, mod.WorkshopID, out)
	if err != nil {
		return fmt.Errorf("mod %s: %w", mod.WorkshopID, err)
	}

	// Backfill metadata from Info.json; a missing Info.json is non-fatal.
	info, err := palmod.ParseInfo(downloadPath)
	if err != nil {
		fmt.Fprintf(out, "==> Warning: mod %s: could not parse Info.json: %v\n", mod.WorkshopID, err)
		// Mark as downloaded even without metadata so callers can deploy the content.
		_ = m.db.Model(&models.Mod{}).Where("id = ?", modID).
			Updates(map[string]any{"downloaded": true, "download_path": downloadPath}).Error
		fmt.Fprintf(out, "==> Download complete (no metadata): %s\n", downloadPath)
		return nil
	}

	if !info.IsServer {
		fmt.Fprintf(out, "==> Warning: mod %s (%s) is not marked IsServer; it may not be designed for dedicated servers\n",
			mod.WorkshopID, info.PackageName)
	}

	if err := m.db.Model(&models.Mod{}).Where("id = ?", modID).
		Select("downloaded", "download_path", "package_name", "mod_name", "version", "tags", "dependencies").
		Updates(models.Mod{
			Downloaded:   true,
			DownloadPath: downloadPath,
			PackageName:  info.PackageName,
			ModName:      info.ModName,
			Version:      info.Version,
			Tags:         info.Tags,
			Dependencies: info.Dependencies,
		}).Error; err != nil {
		return fmt.Errorf("mod %s: persist metadata: %w", mod.WorkshopID, err)
	}

	fmt.Fprintf(out, "==> Download complete: %s\n", downloadPath)
	return nil
}

// DeployServerMods copies every enabled mod from the global staging area into
// the server's Mods/Workshop directory, updates deployed_version on each
// server_mods row, and rewrites PalModSettings.ini. Only mods that are
// downloaded (downloaded=true) and have a resolved DownloadPath are deployed;
// not-yet-downloaded mods produce a warning and are skipped.
//
// Running is allowed: copying into Mods/Workshop does not touch the live
// process; changes take effect on the next restart.
func (m *Manager) DeployServerMods(serverID int64, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}

	m.mu.Lock()
	if _, ok := m.installing[serverID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("server %d is installing", serverID)
	}
	if _, ok := m.deployingMods[serverID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("server %d mods are already deploying", serverID)
	}
	m.deployingMods[serverID] = struct{}{}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.deployingMods, serverID)
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
		return fmt.Errorf("server %d has no install directory", serverID)
	}

	serverMods, globalMods, err := m.loadServerModsWithGlobal(serverID)
	if err != nil {
		return err
	}

	var failures []string
	var enabledForIni []palmod.EnabledMod

	for i := range serverMods {
		sm := &serverMods[i]
		gm, ok := globalMods[sm.ModID]
		if !ok {
			failures = append(failures, fmt.Sprintf("server_mod %d: global mod %d not found", sm.ID, sm.ModID))
			continue
		}

		if !sm.Enabled {
			continue
		}

		if !gm.Downloaded || gm.DownloadPath == "" {
			fmt.Fprintf(out, "==> Skipping mod %s (%s): not yet downloaded\n", gm.WorkshopID, gm.Name)
			continue
		}

		fmt.Fprintf(out, "==> Deploying mod %s (%s) to server...\n", gm.WorkshopID, gm.Name)
		if _, err := palmod.Deploy(installPath, gm.WorkshopID, gm.DownloadPath); err != nil {
			failures = append(failures, fmt.Sprintf("mod %s: deploy: %v", gm.WorkshopID, err))
			continue
		}

		// Update deployed_version to match the global library version.
		if err := m.db.Model(&models.ServerMod{}).Where("id = ?", sm.ID).
			Update("deployed_version", gm.Version).Error; err != nil {
			failures = append(failures, fmt.Sprintf("mod %s: persist deployed_version: %v", gm.WorkshopID, err))
		}
		sm.DeployedVersion = gm.Version

		if strings.TrimSpace(gm.PackageName) != "" {
			enabledForIni = append(enabledForIni, palmod.EnabledMod{PackageName: gm.PackageName})
		}
	}

	m.iniMu.Lock()
	iniErr := palmod.WriteModSettings(installPath, enabledForIni)
	m.iniMu.Unlock()
	if iniErr != nil {
		failures = append(failures, fmt.Sprintf("write PalModSettings.ini: %v", iniErr))
	}

	if len(failures) > 0 {
		msg := "Mod deployment completed with errors:\n" + strings.Join(failures, "\n")
		fmt.Fprintf(out, "==> %s\n", msg)
		return errors.New(msg)
	}

	fmt.Fprintf(out, "==> Mod deployment complete\n")
	return nil
}

// loadServerModsWithGlobal loads all server_mods for a server and returns a
// map of global Mod entries keyed by mod_id. Used by DeployServerMods and
// syncModsOnStart to avoid duplicating the join logic.
func (m *Manager) loadServerModsWithGlobal(serverID int64) ([]models.ServerMod, map[int64]models.Mod, error) {
	var serverMods []models.ServerMod
	if err := m.db.Where("server_id = ?", serverID).Find(&serverMods).Error; err != nil {
		return nil, nil, err
	}
	if len(serverMods) == 0 {
		return serverMods, map[int64]models.Mod{}, nil
	}
	modIDs := make([]int64, len(serverMods))
	for i, sm := range serverMods {
		modIDs[i] = sm.ModID
	}
	var globalMods []models.Mod
	if err := m.db.Where("id IN ?", modIDs).Find(&globalMods).Error; err != nil {
		return nil, nil, err
	}
	modMap := make(map[int64]models.Mod, len(globalMods))
	for _, gm := range globalMods {
		modMap[gm.ID] = gm
	}
	return serverMods, modMap, nil
}

// syncModsOnStart silently re-deploys any enabled mod whose global library
// version differs from the version last deployed to the server. Called from
// StartServer before launching the process; failures are logged as warnings and
// never block the start.
func (m *Manager) syncModsOnStart(serverID int64, installPath string) {
	serverMods, globalMods, err := m.loadServerModsWithGlobal(serverID)
	if err != nil {
		fmt.Printf("warning: server %d mod sync: load mods: %v\n", serverID, err)
		return
	}
	if len(serverMods) == 0 {
		return
	}

	changed := false
	var enabledForIni []palmod.EnabledMod

	for i := range serverMods {
		sm := &serverMods[i]
		gm, ok := globalMods[sm.ModID]
		if !ok {
			continue
		}

		if sm.Enabled && strings.TrimSpace(gm.PackageName) != "" {
			enabledForIni = append(enabledForIni, palmod.EnabledMod{PackageName: gm.PackageName})
		}

		// Only sync enabled mods that have been downloaded and have a version mismatch.
		if !sm.Enabled || !gm.Downloaded || gm.DownloadPath == "" {
			continue
		}
		if gm.Version == "" || gm.Version == sm.DeployedVersion {
			continue
		}

		fmt.Printf("server %d: auto-syncing mod %s (%s → %s)\n",
			serverID, gm.WorkshopID, sm.DeployedVersion, gm.Version)

		if _, err := palmod.Deploy(installPath, gm.WorkshopID, gm.DownloadPath); err != nil {
			fmt.Printf("warning: server %d mod %s auto-sync deploy: %v\n", serverID, gm.WorkshopID, err)
			continue
		}
		if err := m.db.Model(&models.ServerMod{}).Where("id = ?", sm.ID).
			Update("deployed_version", gm.Version).Error; err != nil {
			fmt.Printf("warning: server %d mod %s persist deployed_version: %v\n", serverID, gm.WorkshopID, err)
		}
		changed = true
	}

	if changed || len(enabledForIni) > 0 {
		m.iniMu.Lock()
		if err := palmod.WriteModSettings(installPath, enabledForIni); err != nil {
			fmt.Printf("warning: server %d mod sync: rewrite PalModSettings.ini: %v\n", serverID, err)
		}
		m.iniMu.Unlock()
	}
}

// RewriteModSettings recomputes PalModSettings.ini for a server from its current
// server_mods rows joined with the global mod library. Called after a toggle or
// unlink — neither of which re-downloads — so only the enabled flag changes.
// Only enabled mods with a resolved PackageName are written to ActiveModList.
func (m *Manager) RewriteModSettings(serverID int64) error {
	var srv models.Server
	if err := m.db.Select("install_path").First(&srv, serverID).Error; err != nil {
		return err
	}
	if srv.InstallPath == "" {
		return fmt.Errorf("server %d has no install directory", serverID)
	}

	serverMods, globalMods, err := m.loadServerModsWithGlobal(serverID)
	if err != nil {
		return err
	}

	var enabled []palmod.EnabledMod
	for _, sm := range serverMods {
		if !sm.Enabled {
			continue
		}
		gm, ok := globalMods[sm.ModID]
		if !ok {
			continue
		}
		if strings.TrimSpace(gm.PackageName) != "" {
			enabled = append(enabled, palmod.EnabledMod{PackageName: gm.PackageName})
		}
	}

	m.iniMu.Lock()
	defer m.iniMu.Unlock()
	return palmod.WriteModSettings(srv.InstallPath, enabled)
}
