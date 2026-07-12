package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/steamcmd"
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

// ListServers returns all servers
func (r *Router) ListServers(c *gin.Context) {
	rows, err := r.db.Query("SELECT id, name, install_path, port, query_port, rcon_port, rcon_enabled, status, pid, created_at, updated_at FROM servers ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query servers"})
		return
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var s models.Server
		err := rows.Scan(&s.ID, &s.Name, &s.InstallPath, &s.Port, &s.QueryPort, &s.RCONPort, &s.RCONEnabled, &s.Status, &s.PID, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan server"})
			return
		}
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

	// If no install path provided, we'll use a default based on ID after insertion
	installPath := req.InstallPath

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

	// If no install path was provided, update with default Server/{id}
	if installPath == "" {
		installPath = fmt.Sprintf("Server/%d", id)
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

	var s models.Server
	err = r.db.QueryRow(
		"SELECT id, name, install_path, port, query_port, rcon_port, rcon_enabled, status, pid, created_at, updated_at FROM servers WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.InstallPath, &s.Port, &s.QueryPort, &s.RCONPort, &s.RCONEnabled, &s.Status, &s.PID, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}

	c.JSON(http.StatusOK, s)
}

// UpdateServerRequest represents the request body for updating a server
type UpdateServerRequest struct {
	Name        string `json:"name"`
	Port        int    `json:"port"`
	QueryPort   int    `json:"queryPort"`
	RCONPort    int    `json:"rconPort"`
	RCONEnabled bool   `json:"rconEnabled"`
}

// UpdateServer updates a server
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

	// Check if server exists
	var exists bool
	err = r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM servers WHERE id = ?)", id).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	// Update server
	_, err = r.db.Exec(
		"UPDATE servers SET name = ?, port = ?, query_port = ?, rcon_port = ?, rcon_enabled = ?, updated_at = ? WHERE id = ?",
		req.Name, req.Port, req.QueryPort, req.RCONPort, req.RCONEnabled, time.Now(), id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update server"})
		return
	}

	// Fetch and return updated server
	var s models.Server
	err = r.db.QueryRow(
		"SELECT id, name, install_path, port, query_port, rcon_port, rcon_enabled, status, pid, created_at, updated_at FROM servers WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.InstallPath, &s.Port, &s.QueryPort, &s.RCONPort, &s.RCONEnabled, &s.Status, &s.PID, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query updated server"})
		return
	}

	c.JSON(http.StatusOK, s)
}

// DeleteServer deletes a server
func (r *Router) DeleteServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Check if server exists and get its status
	var status string
	err = r.db.QueryRow("SELECT status FROM servers WHERE id = ?", id).Scan(&status)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}

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

	// Get server info
	var s models.Server
	err = r.db.QueryRow(
		"SELECT id, name, install_path, status FROM servers WHERE id = ?", id,
	).Scan(&s.ID, &s.Name, &s.InstallPath, &s.Status)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server"})
		return
	}

	// Check if server is already installing or running
	if s.Status != "stopped" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server must be stopped to install"})
		return
	}

	// Update status to installing
	_, err = r.db.Exec("UPDATE servers SET status = ?, updated_at = ? WHERE id = ?", "installing", time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update server status"})
		return
	}

	// Start installation in background
	go func() {
		err := steamcmd.InstallPalworldServer(s.InstallPath, r.config.SteamCMDPath)

		// Update status based on installation result
		newStatus := "stopped"
		if err != nil {
			newStatus = "error"
		}

		r.db.Exec("UPDATE servers SET status = ?, updated_at = ? WHERE id = ?", newStatus, time.Now(), id)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Server installation started",
		"serverId": id,
		"status": "installing",
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
