package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/settings"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/steamcmd"
	"github.com/gin-gonic/gin"
)

// steamLoginTimeout bounds a synchronous steamcmd login so a stuck child process
// cannot hang the HTTP request indefinitely.
const steamLoginTimeout = 60 * time.Second

// SteamStatus reports the configured Steam username and whether a cached login
// session is believed ready. This drives the Mods-tab account UI (not
// configured / logged in / needs login).
func (r *Router) SteamStatus(c *gin.Context) {
	username, err := settings.Get(r.db, settings.KeySteamUsername)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read settings"})
		return
	}
	// Surface the config default when nothing has been set at runtime yet.
	if strings.TrimSpace(username) == "" {
		username = strings.TrimSpace(r.config.SteamUsername)
	}

	ready, err := settings.Get(r.db, settings.KeySteamSessionReady)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":     username,
		"sessionReady": ready == "true",
	})
}

// SteamLoginRequest is the body for POST /api/steam/login.
//
// SECURITY: Password is used only for this single login call. It is never
// persisted, logged, added to last_error, or echoed back in the response.
type SteamLoginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	GuardCode string `json:"guardCode"`
}

// SteamLogin runs steamcmd +login synchronously and classifies the result. On
// success it persists the username and marks the session ready so subsequent
// workshop downloads reuse the cached SteamCMD session. The password is dropped
// as soon as steamcmd returns and never leaves this function.
func (r *Router) SteamLogin(c *gin.Context) {
	var req SteamLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), steamLoginTimeout)
	defer cancel()

	// out=nil: steamcmd's login output is discarded, not persisted. The password
	// is not echoed by steamcmd, but discarding output is defense-in-depth.
	result, _ := steamcmd.Login(ctx, r.config.SteamCMDPath, username, req.Password, req.GuardCode, nil)

	// Drop the password reference immediately after the login call returns.
	req.Password = ""

	switch result {
	case steamcmd.LoginSuccess:
		if err := settings.Set(r.db, settings.KeySteamUsername, username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Steam username"})
			return
		}
		if err := settings.Set(r.db, settings.KeySteamSessionReady, "true"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session state"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"result": "success", "message": "Steam login successful"})
	case steamcmd.LoginNeedGuard:
		c.JSON(http.StatusOK, gin.H{"result": "needGuard", "message": "Steam Guard code required"})
	case steamcmd.LoginBadCredentials:
		c.JSON(http.StatusOK, gin.H{"result": "badCredentials", "message": "Invalid username or password"})
	default:
		c.JSON(http.StatusOK, gin.H{"result": "error", "message": "Steam login failed"})
	}
}
