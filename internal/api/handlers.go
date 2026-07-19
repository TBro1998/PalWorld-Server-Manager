package api

import (
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palconfig"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/process"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Note: Login, Register, Setup, and AuthStatus handlers are in auth_handlers.go.

// Creation-time port defaults. The first server uses these values; each
// additional server increments each value by 1 based on the previous server.
const (
	firstGamePort    = 8211
	firstQueryPort   = 27015
	firstRESTAPIPort = 8311
)

// randomAdminPassword generates a 12-character random alphanumeric password
// using crypto/rand for use as the initial AdminPassword on new servers.
func randomAdminPassword() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	raw := make([]byte, 12)
	if _, err := cryptorand.Read(raw); err != nil {
		return "PalAdmin1234" // safe fallback; crypto/rand failure is extremely rare
	}
	out := make([]byte, 12)
	for i, b := range raw {
		out[i] = charset[int(b)%len(charset)]
	}
	return string(out)
}

// lastServerPorts returns the game port, Steam query port, and REST API port
// of the most recently created server so the next server can default to each
// plus one. Returns the first-server defaults when no servers exist yet.
func (r *Router) lastServerPorts() (gamePort, queryPort, restAPIPort int) {
	gamePort, queryPort, restAPIPort = firstGamePort, firstQueryPort, firstRESTAPIPort
	var last models.Server
	if err := r.db.Order("created_at DESC").Select("launch_args").First(&last).Error; err != nil {
		return // no servers yet — use first-server defaults
	}
	args, err := palconfig.ParseLaunchArgs(last.LaunchArgs)
	if err != nil {
		return
	}
	if args.Port != nil {
		gamePort = *args.Port + 1
	}
	if args.QueryPort != nil {
		queryPort = *args.QueryPort + 1
	}
	if args.RESTAPIPort != nil {
		restAPIPort = *args.RESTAPIPort + 1
	}
	return
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

// ListServers godoc
// @Summary      List all servers
// @Description  Returns all server records with derived status
// @Tags         servers
// @Produce      json
// @Success      200  {array}   models.Server
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers [get]
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

// CreateServer godoc
// @Summary      Create a new server
// @Description  Creates a server record with auto-incremented ports and random admin password
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        body  body      CreateServerRequest  true  "Server name and optional install path"
// @Success      201   {object}  models.Server
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers [post]
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

	// Pre-compute per-server port defaults (+1 from the most recently created
	// server) and generate a random admin password. These are stored in
	// launch_args and used by GetServerConfig to pre-fill the config form
	// before the server is installed.
	gamePort, queryPort, restAPIPort := r.lastServerPorts()
	adminPass := randomAdminPassword()
	initArgs := palconfig.LaunchArgs{
		Port:                 &gamePort,
		QueryPort:            &queryPort,
		RESTAPIPort:          &restAPIPort,
		InitialAdminPassword: adminPass,
	}
	argsJSON, _ := initArgs.Marshal()

	// Status is never persisted as a truth source (it is derived); the model omits
	// it (gorm:"-"). GORM auto-populates ID, created_at and updated_at on Create.
	server := models.Server{
		Name:        req.Name,
		InstallPath: installPath,
		PID:         0,
		LaunchArgs:  argsJSON,
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

	// Seed PalWorldSettings.ini with the creation-time presets immediately,
	// before the server is installed. SteamCMD writes game binaries into
	// installPath/ but never touches Pal/Saved/Config/*, so this file will
	// survive installation intact. Without this step, ensureFile would later
	// seed from the game-shipped DefaultPalWorldSettings.ini (which uses
	// hardcoded defaults) and overwrite our RESTAPIPort/AdminPassword presets.
	seedSettings := palconfig.Defaults()
	seedSettings["RESTAPIEnabled"] = "True"
	seedSettings["RESTAPIPort"] = strconv.Itoa(restAPIPort)
	seedSettings["AdminPassword"] = adminPass
	if err := palconfig.SaveSettings(installPath, seedSettings); err != nil {
		// Non-fatal: log the warning but do not block server creation. The
		// presets remain available via launch_args for the uninstalled view.
		fmt.Printf("warning: could not seed PalWorldSettings.ini for server %d: %v\n", server.ID, err)
	}

	// Return the created server with its derived status.
	server.Status = "stopped"
	c.JSON(http.StatusCreated, server)
}

// GetServer returns a specific server
// GetServer godoc
// @Summary      Get server by ID
// @Description  Returns a single server record with derived status
// @Tags         servers
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  models.Server
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id} [get]
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

// UpdateServer godoc
// @Summary      Update server metadata
// @Description  Updates server name, install path, and/or launch arguments
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        id    path      int                  true  "Server ID"
// @Param        body  body      UpdateServerRequest  true  "Fields to update"
// @Success      200   {object}  models.Server
// @Failure      400   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id} [put]
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
// DeleteServer godoc
// @Summary      Delete a server
// @Description  Deletes a server record. Does not remove game files from disk.
// @Tags         servers
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}  "Server is running"
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id} [delete]
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
// InstallServer godoc
// @Summary      Install server files via SteamCMD
// @Description  Downloads Palworld dedicated server files to the install path. May take several minutes.
// @Tags         servers
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}  "Installation started"
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}  "Already installed or running"
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/install [post]
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
// StartServer godoc
// @Summary      Start server
// @Description  Starts the Palworld dedicated server process
// @Tags         servers
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}  "Already running or not installed"
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/start [post]
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
// StopServer godoc
// @Summary      Stop server
// @Description  Gracefully stops the running server process. Disconnects all online players.
// @Tags         servers
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}  "Not running"
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/stop [post]
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
// RestartServer godoc
// @Summary      Restart server
// @Description  Stops and restarts the server process. Disconnects all online players.
// @Tags         servers
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}  "Not running or not installed"
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/restart [post]
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
// GetLogs godoc
// @Summary      Get server logs
// @Description  Returns recent log lines. Use query param ?limit=N to control count (default 100).
// @Tags         servers
// @Produce      json
// @Param        id     path      int     true   "Server ID"
// @Param        limit  query     int     false  "Number of recent lines (default 100)"
// @Success      200    {object}  map[string]interface{}  "lines: array of strings"
// @Failure      400    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/logs [get]
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
// StreamLogs godoc
// @Summary      Stream server logs (SSE)
// @Description  Server-Sent Events stream of live log lines. Auth via ?token=<jwt> query param.
// @Tags         servers
// @Produce      text/event-stream
// @Param        id     path      int     true   "Server ID"
// @Param        token  query     string  false  "JWT token (for SSE auth)"
// @Success      200    {string}  string  "SSE stream"
// @Failure      400    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/logs/stream [get]
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
// GetServerConfig godoc
// @Summary      Get server configuration
// @Description  Returns PalWorldSettings.ini and launch arguments for editing
// @Tags         config
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}  "settings and launchArgs"
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/config [get]
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

	// Before the server is installed (or when no install path is set), return
	// schema defaults augmented with the creation-time port/password presets
	// stored in launch_args. This lets the config form show the correct
	// per-server defaults without touching disk.
	if !installed || installPath == "" {
		defaults := palconfig.Defaults()
		if launchArgs.RESTAPIPort != nil {
			defaults["RESTAPIPort"] = strconv.Itoa(*launchArgs.RESTAPIPort)
		}
		if launchArgs.InitialAdminPassword != "" {
			defaults["AdminPassword"] = launchArgs.InitialAdminPassword
		}
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
// optionally updates launch args. Allowed at any time; changes take effect on
// the next server start because Palworld reads the INI only at startup.
// UpdateServerConfig godoc
// @Summary      Update server configuration
// @Description  Writes PalWorldSettings.ini and/or launch arguments. Server must be stopped.
// @Tags         config
// @Accept       json
// @Produce      json
// @Param        id    path      int                   true  "Server ID"
// @Param        body  body      map[string]interface{}  true  "settings and/or launchArgs"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      409   {object}  map[string]interface{}  "Server is running"
// @Failure      500   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/config [put]
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

	installPath, _, _, _, err := r.loadServerPathState(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
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
// GetConfigSchema godoc
// @Summary      Get configuration schema
// @Description  Returns metadata for all PalWorldSettings.ini fields (types, defaults, descriptions)
// @Tags         config
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /config/schema [get]
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

// GetSystemStats returns system statistics
// GetSystemStats godoc
// @Summary      Get system monitoring stats
// @Description  Returns CPU, memory, and disk usage
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /system/stats [get]
func (r *Router) GetSystemStats(c *gin.Context) {
	// TODO: Implement system stats logic
	c.JSON(http.StatusNotImplemented, gin.H{"message": "System stats - to be implemented"})
}
