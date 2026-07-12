package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palconfig"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/process"
	"github.com/gin-gonic/gin"
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

const serverColumns = "id, name, install_path, port, query_port, rcon_port, rcon_enabled, last_error, pid, launch_args, installed, created_at, updated_at"

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

func scanServer(sc interface {
	Scan(dest ...any) error
}) (models.Server, error) {
	var s models.Server
	err := sc.Scan(&s.ID, &s.Name, &s.InstallPath, &s.Port, &s.QueryPort, &s.RCONPort, &s.RCONEnabled, &s.LastError, &s.PID, &s.LaunchArgs, &s.Installed, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// hydrateStatus fills the derived Status field from in-memory manager state and
// the persisted last_error. Status is never read from the database.
func (r *Router) hydrateStatus(s *models.Server) {
	s.Status = r.process.DeriveStatus(s.ID, s.LastError)
}

// ListServers returns all servers
func (r *Router) ListServers(c *gin.Context) {
	rows, err := r.db.Query("SELECT " + serverColumns + " FROM servers ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query servers"})
		return
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan server"})
			return
		}
		r.hydrateStatus(&s)
		servers = append(servers, s)
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

	now := time.Now()
	result, err := r.db.Exec(
		"INSERT INTO servers (name, install_path, port, query_port, rcon_port, rcon_enabled, status, pid, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		req.Name, installPath, 8211, 27015, 25575, false, "stopped", 0, now, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create server"})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get server ID"})
		return
	}

	// If no install path was provided, update with default Servers/{id}
	if installPath == "" {
		installPath = absInstallPath(fmt.Sprintf("Servers/%d", id))
		_, err = r.db.Exec("UPDATE servers SET install_path = ? WHERE id = ?", installPath, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update install path"})
			return
		}
	}

	// Return the created server
	server := models.Server{
		ID:          id,
		Name:        req.Name,
		InstallPath: installPath,
		Port:        8211,
		QueryPort:   27015,
		RCONPort:    25575,
		RCONEnabled: false,
		Status:      "stopped",
		PID:         0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	c.JSON(http.StatusCreated, server)
}

// GetServer returns a specific server
func (r *Router) GetServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	s, err := scanServer(r.db.QueryRow("SELECT "+serverColumns+" FROM servers WHERE id = ?", id))
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
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
	Port        int              `json:"port"`
	QueryPort   int              `json:"queryPort"`
	RCONPort    int              `json:"rconPort"`
	RCONEnabled bool             `json:"rconEnabled"`
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
	var curPath, lastError string
	err = r.db.QueryRow("SELECT install_path, last_error FROM servers WHERE id = ?", id).Scan(&curPath, &lastError)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}
	status := r.process.DeriveStatus(id, lastError)

	now := time.Now()

	// Base metadata always updates.
	if _, err = r.db.Exec(
		"UPDATE servers SET name = ?, port = ?, query_port = ?, rcon_port = ?, rcon_enabled = ?, updated_at = ? WHERE id = ?",
		req.Name, req.Port, req.QueryPort, req.RCONPort, req.RCONEnabled, now, id,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update server"})
		return
	}

	// Install directory change: only allowed while stopped; recompute installed.
	// Canonicalize to an absolute path before comparing/storing so a relative
	// input that resolves to the current directory is treated as "no change".
	if req.InstallPath != nil {
		newPath := absInstallPath(*req.InstallPath)
		if newPath != curPath {
			if status != "stopped" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change install directory while server is " + status})
				return
			}
			installed := process.IsInstalled(newPath)
			if _, err = r.db.Exec(
				"UPDATE servers SET install_path = ?, installed = ?, updated_at = ? WHERE id = ?",
				newPath, installed, now, id,
			); err != nil {
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
		if _, err = r.db.Exec(
			"UPDATE servers SET launch_args = ?, updated_at = ? WHERE id = ?",
			canonical, now, id,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update launch args"})
			return
		}
	}

	s, err := scanServer(r.db.QueryRow("SELECT "+serverColumns+" FROM servers WHERE id = ?", id))
	if err != nil {
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
	var lastError string
	err = r.db.QueryRow("SELECT last_error FROM servers WHERE id = ?", id).Scan(&lastError)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}
	status := r.process.DeriveStatus(id, lastError)

	// Prevent deletion of running or installing servers
	if status == "running" || status == "installing" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete server while it is " + status})
		return
	}

	// Delete server
	_, err = r.db.Exec("DELETE FROM servers WHERE id = ?", id)
	if err != nil {
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
	var exists int
	err = r.db.QueryRow("SELECT 1 FROM servers WHERE id = ?", id).Scan(&exists)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
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

	// Start installation in background. The Manager owns the installing set and
	// persists facts (last_error, installed); status is never written.
	go func() {
		// Compose log sinks: persist to disk and broadcast live lines to SSE
		// clients, mirroring the server-process logging pipeline.
		capture := logger.NewCapture(id, r.config.LogDir)
		broadcaster := logger.NewBroadcastWriter(r.streams, id)
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

// GetLogs returns the most recent server logs.
// Optional query param: lines (default 200).
func (r *Router) GetLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	lines := 200
	if v := c.Query("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lines = n
		}
	}

	logs, err := logger.ReadLogs(r.config.LogDir, id, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"serverId": id,
		"logs":     logs,
	})
}

// StreamLogs streams live server logs via Server-Sent Events.
func (r *Router) StreamLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	clientID, ch := r.streams.Subscribe(id)
	defer r.streams.Unsubscribe(id, clientID)

	ctx := c.Request.Context()
	c.Stream(func(w io.Writer) bool {
		select {
		case line, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent("log", line)
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
	err = r.db.QueryRow(
		"SELECT install_path, last_error, launch_args, installed FROM servers WHERE id = ?", id,
	).Scan(&installPath, &lastError, &launchArgs, &installed)
	return
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
		if err.Error() == "sql: no rows in result set" {
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
		if err.Error() == "sql: no rows in result set" {
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
		if _, err := r.db.Exec(
			"UPDATE servers SET launch_args = ?, updated_at = ? WHERE id = ?",
			canonical, time.Now(), id,
		); err != nil {
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

// ListMods returns all mods for a server
func (r *Router) ListMods(c *gin.Context) {
	// TODO: Implement list mods logic
	c.JSON(http.StatusOK, gin.H{"mods": []interface{}{}})
}

// InstallMod installs a mod
func (r *Router) InstallMod(c *gin.Context) {
	// TODO: Implement install mod logic
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Install mod - to be implemented"})
}

// UninstallMod uninstalls a mod
func (r *Router) UninstallMod(c *gin.Context) {
	// TODO: Implement uninstall mod logic
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Uninstall mod - to be implemented"})
}

// ToggleMod enables/disables a mod
func (r *Router) ToggleMod(c *gin.Context) {
	// TODO: Implement toggle mod logic
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Toggle mod - to be implemented"})
}

// GetSystemStats returns system statistics
func (r *Router) GetSystemStats(c *gin.Context) {
	// TODO: Implement system stats logic
	c.JSON(http.StatusNotImplemented, gin.H{"message": "System stats - to be implemented"})
}
