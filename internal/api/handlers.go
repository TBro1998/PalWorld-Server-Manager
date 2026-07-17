package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palconfig"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palmod"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/process"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Login handles user login
func (r *Router) Login(c *gin.Context) {
	// TODO: Implement login logic
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Login endpoint - to be implemented"})
}

// Register handles user registration
func (r *Router) Register(c *gin.Context) {
	// TODO: Implement registration logic
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Register endpoint - to be implemented"})
}

// absInstallPath normalizes a server install path to an absolute path. SteamCMD's
// +force_install_dir and the server launch working directory both require an
// absolute path to behave predictably, so we canonicalize before persisting.
// Empty input is returned as-is (callers treat "" as "not yet assigned"); if the
// path cannot be resolved, the original value is returned unchanged.
func absInstallPath(p string) string {
	if p == "" {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

// hydrateStatus fills the derived Status field from in-memory manager state and
// the persisted last_error. Status is never read from the database.
func (r *Router) hydrateStatus(s *models.Server) {
	s.Status = r.process.DeriveStatus(s.ID, s.LastError)
}

// ListServers returns all servers
func (r *Router) ListServers(c *gin.Context) {
	var servers []models.Server
	if err := r.db.Order("created_at DESC").Find(&servers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query servers"})
		return
	}

	for i := range servers {
		r.hydrateStatus(&servers[i])
	}

	if servers == nil {
		servers = []models.Server{}
	}

	c.JSON(http.StatusOK, servers)
}

// CreateServerRequest represents the request body for creating a server
type CreateServerRequest struct {
	Name        string `json:"name" binding:"required"`
	InstallPath string `json:"installPath"`
}

// CreateServer creates a new server
func (r *Router) CreateServer(c *gin.Context) {
	var req CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If no install path provided, we'll use a default based on ID after insertion.
	// User-supplied paths are canonicalized to absolute so downstream consumers
	// (SteamCMD, launch cwd, config I/O) always see a stable, absolute location.
	installPath := absInstallPath(req.InstallPath)

	// Status is never persisted as a truth source (it is derived); the model omits
	// it (gorm:"-"). GORM auto-populates ID, created_at and updated_at on Create.
	server := models.Server{
		Name:        req.Name,
		InstallPath: installPath,
		PID:         0,
	}
	if err := r.db.Create(&server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create server"})
		return
	}

	// If no install path was provided, update with default Servers/{id}
	if installPath == "" {
		installPath = absInstallPath(fmt.Sprintf("Servers/%d", server.ID))
		if err := r.db.Model(&server).Update("install_path", installPath).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update install path"})
			return
		}
	}

	// Return the created server with its derived status.
	server.Status = "stopped"
	c.JSON(http.StatusCreated, server)
}

// GetServer returns a specific server
func (r *Router) GetServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	var s models.Server
	if err := r.db.First(&s, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}

	r.hydrateStatus(&s)
	c.JSON(http.StatusOK, s)
}

// UpdateServerRequest represents the request body for updating a server.
// InstallPath and LaunchArgs are optional (pointer) so clients can omit them.
type UpdateServerRequest struct {
	Name        string           `json:"name"`
	InstallPath *string          `json:"installPath,omitempty"`
	LaunchArgs  *json.RawMessage `json:"launchArgs,omitempty"`
}

// UpdateServer updates a server's metadata and, optionally, its install
// directory and launch arguments.
func (r *Router) UpdateServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	var req UpdateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Load current directory and last error, then derive status.
	var cur models.Server
	if err := r.db.Select("install_path", "last_error").First(&cur, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}
	status := r.process.DeriveStatus(id, cur.LastError)

	// Base metadata always updates. Single-column Update auto-refreshes updated_at.
	if err := r.db.Model(&models.Server{}).Where("id = ?", id).
		Update("name", req.Name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update server"})
		return
	}

	// Install directory change: only allowed while stopped; recompute installed.
	// Canonicalize to an absolute path before comparing/storing so a relative
	// input that resolves to the current directory is treated as "no change".
	if req.InstallPath != nil {
		newPath := absInstallPath(*req.InstallPath)
		if newPath != cur.InstallPath {
			if status != "stopped" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change install directory while server is " + status})
				return
			}
			installed := process.IsInstalled(newPath)
			if err := r.db.Model(&models.Server{}).Where("id = ?", id).
				Updates(map[string]any{"install_path": newPath, "installed": installed}).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update install path"})
				return
			}
		}
	}

	// Launch arguments: validate then store canonical JSON.
	if req.LaunchArgs != nil {
		parsed, perr := palconfig.ParseLaunchArgs(string(*req.LaunchArgs))
		if perr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": perr.Error()})
			return
		}
		canonical, _ := parsed.Marshal()
		if err := r.db.Model(&models.Server{}).Where("id = ?", id).
			Update("launch_args", canonical).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update launch args"})
			return
		}
	}

	var s models.Server
	if err := r.db.First(&s, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query updated server"})
		return
	}
	r.hydrateStatus(&s)
	c.JSON(http.StatusOK, s)
}

// DeleteServer deletes a server
func (r *Router) DeleteServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Check if server exists and derive its status.
	var srv models.Server
	if err := r.db.Select("last_error").First(&srv, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}
	status := r.process.DeriveStatus(id, srv.LastError)

	// Prevent deletion of running or installing servers
	if status == "running" || status == "installing" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete server while it is " + status})
		return
	}

	// Delete server. The model has no DeletedAt field, so this is a hard delete.
	if err := r.db.Delete(&models.Server{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete server"})
		return
	}

	c.Status(http.StatusNoContent)
}

// InstallServer installs Palworld server files using SteamCMD
func (r *Router) InstallServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Verify the server exists.
	var srv models.Server
	if err := r.db.Select("id").First(&srv, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}

	// Only block while running or already installing; stopped/error may install.
	if r.process.IsRunning(id) || r.process.IsInstalling(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server is running or installing"})
		return
	}

	// Clear the previous run's SteamCMD log so each install/update starts fresh;
	// the prior output is not retained. Done synchronously before responding so a
	// log fetch triggered by the client sees the cleared file.
	if err := logger.ResetLog(r.config.LogDir, id, logger.KindSteamCMD); err != nil {
		fmt.Printf("warning: failed to reset steamcmd log for server %d: %v\n", id, err)
	}

	// Start installation in background. The Manager owns the installing set and
	// persists facts (last_error, installed); status is never written.
	go func() {
		// Compose log sinks: persist to disk and broadcast live lines to SSE
		// clients, mirroring the server-process logging pipeline. KindSteamCMD
		// keeps install/update output on its own file and channel, separate from
		// the running server's logs.
		capture := logger.NewCapture(id, logger.KindSteamCMD, r.config.LogDir)
		broadcaster := logger.NewBroadcastWriter(r.streams, id, logger.KindSteamCMD)
		out := io.MultiWriter(capture, broadcaster)
		defer capture.Close()

		if err := r.process.InstallServer(id, out); err != nil {
			fmt.Printf("server %d install failed: %v\n", id, err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":  "Server installation started",
		"serverId": id,
		"status":   "installing",
	})
}

// StartServer starts a server
func (r *Router) StartServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	if err := r.process.StartServer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Server started",
		"serverId": id,
		"status":   "running",
	})
}

// StopServer stops a server
func (r *Router) StopServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	if err := r.process.StopServer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Server stopped",
		"serverId": id,
		"status":   "stopped",
	})
}

// RestartServer restarts a server
func (r *Router) RestartServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	if err := r.process.RestartServer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Server restarted",
		"serverId": id,
		"status":   "running",
	})
}

// logKind resolves the ?kind= query param to a valid log kind, defaulting to
// KindServer. The second return value is false for an unrecognized kind.
func logKind(c *gin.Context) (string, bool) {
	switch c.Query("kind") {
	case "", logger.KindServer:
		return logger.KindServer, true
	case logger.KindSteamCMD:
		return logger.KindSteamCMD, true
	default:
		return "", false
	}
}

// GetLogs returns the most recent server logs.
// Optional query params: kind (server|steamcmd, default server), lines (default 200).
func (r *Router) GetLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	kind, ok := logKind(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log kind"})
		return
	}

	lines := 200
	if v := c.Query("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lines = n
		}
	}

	logs, err := logger.ReadLogs(r.config.LogDir, id, kind, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"serverId": id,
		"kind":     kind,
		"logs":     logs,
	})
}

// StreamLogs streams live server logs via Server-Sent Events.
// Optional query param: kind (server|steamcmd, default server).
func (r *Router) StreamLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	kind, ok := logKind(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log kind"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	clientID, ch := r.streams.Subscribe(id, kind)
	defer r.streams.Unsubscribe(id, kind, clientID)

	ctx := c.Request.Context()
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent(msg.Event, msg.Data)
			return true
		case <-ctx.Done():
			return false
		}
	})
}

// ServerConfigResponse is returned by GetServerConfig.
type ServerConfigResponse struct {
	Settings   map[string]string    `json:"settings"`
	LaunchArgs palconfig.LaunchArgs `json:"launchArgs"`
	Raw        string               `json:"raw"`
	Installed  bool                 `json:"installed"`
}

// UpdateServerConfigRequest is the body for PUT /servers/:id/config.
// Provide either Settings (structured) or Raw (verbatim OptionSettings line);
// LaunchArgs is optional and applies in both modes.
type UpdateServerConfigRequest struct {
	Settings   map[string]string     `json:"settings,omitempty"`
	Raw        *string               `json:"raw,omitempty"`
	LaunchArgs *palconfig.LaunchArgs `json:"launchArgs,omitempty"`
}

func (r *Router) loadServerPathState(id int64) (installPath, lastError, launchArgs string, installed bool, err error) {
	var s models.Server
	err = r.db.Select("install_path", "last_error", "launch_args", "installed").First(&s, id).Error
	if err != nil {
		return
	}
	return s.InstallPath, s.LastError, s.LaunchArgs, s.Installed, nil
}

// GetServerConfig returns the effective PalWorldSettings values, launch args,
// and the raw OptionSettings line. Seeds the INI from defaults when needed.
func (r *Router) GetServerConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	installPath, _, launchArgsJSON, installed, err := r.loadServerPathState(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}

	launchArgs, _ := palconfig.ParseLaunchArgs(launchArgsJSON)

	// Without an install path we cannot touch disk; return registry defaults.
	if installPath == "" {
		defaults := palconfig.Defaults()
		c.JSON(http.StatusOK, ServerConfigResponse{
			Settings:   defaults,
			LaunchArgs: launchArgs,
			Raw:        palconfig.RawLine(defaults),
			Installed:  installed,
		})
		return
	}

	settings, err := palconfig.LoadSettings(installPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	raw, err := palconfig.LoadRaw(installPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ServerConfigResponse{
		Settings:   settings,
		LaunchArgs: launchArgs,
		Raw:        raw,
		Installed:  installed,
	})
}

// UpdateServerConfig writes PalWorldSettings.ini (structured or raw) and
// optionally updates launch args. Only allowed while the server is stopped.
func (r *Router) UpdateServerConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	var req UpdateServerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	installPath, lastError, _, _, err := r.loadServerPathState(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}
	status := r.process.DeriveStatus(id, lastError)

	if status != "stopped" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot edit config while server is " + status})
		return
	}
	if installPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Set an install directory before editing config"})
		return
	}

	// Write INI (raw takes precedence over structured settings).
	if req.Raw != nil {
		if err := palconfig.SaveRaw(installPath, *req.Raw); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else if req.Settings != nil {
		if err := palconfig.SaveSettings(installPath, req.Settings); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Optional launch args.
	if req.LaunchArgs != nil {
		canonical, _ := req.LaunchArgs.Marshal()
		if err := r.db.Model(&models.Server{}).Where("id = ?", id).
			Update("launch_args", canonical).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update launch args"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Config saved", "serverId": id})
}

// GetConfigSchema returns the OptionSettings parameter registry that drives the
// structured config form (keys, types, defaults, categories, enum options).
func (r *Router) GetConfigSchema(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"params": palconfig.Params})
}

// serverExists reports whether a server row exists, writing the appropriate
// error response (404/500) and returning false when it does not.
func (r *Router) serverExists(c *gin.Context, id int64) bool {
	var srv models.Server
	if err := r.db.Select("id").First(&srv, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return false
	}
	return true
}

// loadOwnedMod loads a mod by id and verifies it belongs to serverID (guards
// against cross-server operations). It writes the error response and returns
// nil when the mod is missing or owned by another server.
func (r *Router) loadOwnedMod(c *gin.Context, serverID, modID int64) *models.Mod {
	var mod models.Mod
	if err := r.db.First(&mod, modID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mod not found"})
			return nil
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query mod"})
		return nil
	}
	if mod.ServerID != serverID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mod not found"})
		return nil
	}
	return &mod
}

// ListMods returns all mods registered for a server (by server_id).
func (r *Router) ListMods(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	if !r.serverExists(c, id) {
		return
	}

	var mods []models.Mod
	if err := r.db.Where("server_id = ?", id).Order("created_at ASC").Find(&mods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query mods"})
		return
	}
	if mods == nil {
		mods = []models.Mod{}
	}
	c.JSON(http.StatusOK, gin.H{"mods": mods})
}

// InstallModRequest is the body for POST /servers/:id/mods. It only registers a
// list entry; the actual download happens later via UpdateMods.
type InstallModRequest struct {
	WorkshopID string `json:"workshopId" binding:"required"`
	Name       string `json:"name"`
}

// InstallMod adds a mod entry to the server's list (no download). Metadata
// (package_name/version) is backfilled later by UpdateMods.
func (r *Router) InstallMod(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	if !r.serverExists(c, id) {
		return
	}

	var req InstallModRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.WorkshopID = strings.TrimSpace(req.WorkshopID)
	if req.WorkshopID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workshopId is required"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = req.WorkshopID
	}

	mod := models.Mod{
		ServerID:   id,
		WorkshopID: req.WorkshopID,
		Name:       name,
		Enabled:    true,
	}
	if err := r.db.Create(&mod).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create mod"})
		return
	}
	c.JSON(http.StatusCreated, mod)
}

// UpdateMods downloads/deploys every mod for the server via SteamCMD and
// rewrites the load configuration. It reuses the InstallServer async + logging
// pipeline (KindSteamCMD capture + SSE broadcast); the client observes progress
// via the existing /servers/:id/logs/stream?kind=steamcmd endpoint.
func (r *Router) UpdateMods(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	if !r.serverExists(c, id) {
		return
	}

	// Refuse if installing or already updating (running is allowed: copying into
	// Mods/Workshop does not touch the live process; it takes effect on restart).
	if r.process.IsInstalling(id) || r.process.IsUpdatingMods(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server is installing or mods are already updating"})
		return
	}

	// Clear the previous SteamCMD log so this run starts fresh (done before
	// responding so a client log fetch sees the cleared file).
	if err := logger.ResetLog(r.config.LogDir, id, logger.KindSteamCMD); err != nil {
		fmt.Printf("warning: failed to reset steamcmd log for server %d: %v\n", id, err)
	}

	go func() {
		capture := logger.NewCapture(id, logger.KindSteamCMD, r.config.LogDir)
		broadcaster := logger.NewBroadcastWriter(r.streams, id, logger.KindSteamCMD)
		out := io.MultiWriter(capture, broadcaster)
		defer capture.Close()

		// UpdateMods returns nil only on full success (some mods failing yields a
		// non-nil aggregate error). Broadcast a terminal "done" event carrying that
		// outcome so the live log subscriber can close/refresh without polling.
		result := "ok"
		if err := r.process.UpdateMods(id, out); err != nil {
			result = "error"
			fmt.Printf("server %d mod update failed: %v\n", id, err)
		}
		r.streams.BroadcastEvent(id, logger.KindSteamCMD, "done", result)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":  "Mod update started",
		"serverId": id,
		"status":   "updating",
	})
}

// UninstallMod removes a mod entry: deletes its deployed Workshop directory,
// deletes the DB row, and rewrites PalModSettings.ini.
func (r *Router) UninstallMod(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	modID, err := strconv.ParseInt(c.Param("modId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mod ID"})
		return
	}

	mod := r.loadOwnedMod(c, id, modID)
	if mod == nil {
		return
	}

	// Look up the install path to remove the deployed content (best-effort).
	var srv models.Server
	if err := r.db.Select("install_path").First(&srv, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}
	if srv.InstallPath != "" {
		if err := palmod.Remove(srv.InstallPath, mod.WorkshopID); err != nil {
			fmt.Printf("warning: failed to remove mod dir for server %d mod %s: %v\n", id, mod.WorkshopID, err)
		}
	}

	if err := r.db.Delete(&models.Mod{}, modID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete mod"})
		return
	}

	// Rewrite the load config so the removed mod no longer appears in ActiveModList.
	if srv.InstallPath != "" {
		if err := r.process.RewriteModSettings(id); err != nil {
			fmt.Printf("warning: failed to rewrite PalModSettings.ini for server %d: %v\n", id, err)
		}
	}

	c.Status(http.StatusNoContent)
}

// ToggleMod flips a mod's enabled flag and rewrites PalModSettings.ini. It never
// re-downloads or removes files.
func (r *Router) ToggleMod(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	modID, err := strconv.ParseInt(c.Param("modId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mod ID"})
		return
	}

	mod := r.loadOwnedMod(c, id, modID)
	if mod == nil {
		return
	}

	newEnabled := !mod.Enabled
	if err := r.db.Model(&models.Mod{}).Where("id = ?", modID).
		Update("enabled", newEnabled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update mod"})
		return
	}
	mod.Enabled = newEnabled

	// Rewrite the load config (best-effort; needs an install path to write).
	var srv models.Server
	if err := r.db.Select("install_path").First(&srv, id).Error; err == nil && srv.InstallPath != "" {
		if err := r.process.RewriteModSettings(id); err != nil {
			fmt.Printf("warning: failed to rewrite PalModSettings.ini for server %d: %v\n", id, err)
		}
	}

	c.JSON(http.StatusOK, mod)
}

// GetSystemStats returns system statistics
func (r *Router) GetSystemStats(c *gin.Context) {
	// TODO: Implement system stats logic
	c.JSON(http.StatusNotImplemented, gin.H{"message": "System stats - to be implemented"})
}
