package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/settings"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/steamcmd"
	"github.com/gin-gonic/gin"
)

// steamLoginTimeout bounds a synchronous steamcmd login so a stuck child process
// cannot hang the HTTP request indefinitely. It is generous because a Steam
// Guard mobile-authenticator login blocks on the user approving the request in
// the Steam mobile app ("Waiting for confirmation..."); the user needs time to
// pick up their phone. The frontend axios client sets no timeout, so it waits
// for this to resolve.
const steamLoginTimeout = 180 * time.Second

// steamLogStreamID is the sentinel stream ID used to broadcast steamcmd login
// output over the shared StreamManager. Real server IDs are >= 1, so 0 never
// collides with a per-server stream.
const steamLogStreamID int64 = 0

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
// persisted, logged, added to last_error, or echoed back in the response. The
// steamcmd output IS broadcast live to SSE subscribers (so the user can watch
// the login progress), but steamcmd never echoes the password — it only appears
// in the child process argv, which is not written to that output.
type SteamLoginRequest struct {
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	GuardCode string `json:"guardCode"`
}

// SteamLogin runs steamcmd +login synchronously and classifies the result. On
// success it persists the username and marks the session ready so subsequent
// workshop downloads reuse the cached SteamCMD session. The password is dropped
// as soon as steamcmd returns and never leaves this function.
//
// The steamcmd output is streamed live, line by line, to SSE subscribers via the
// shared StreamManager on the sentinel steamLogStreamID / KindSteamCMD channel;
// it is broadcast only (never written to disk). The HTTP response carries only
// the classified {result, message} signal, not the log.
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

	// Broadcast steamcmd's output live to SSE subscribers of the sentinel login
	// stream (see SECURITY note on SteamLoginRequest: the password is never in
	// this output). Login output is transient — broadcast only, not captured to
	// disk.
	out := logger.NewBroadcastWriter(r.streams, steamLogStreamID, logger.KindSteamCMD)
	result, _ := steamcmd.Login(ctx, r.config.SteamCMDPath, username, req.Password, req.GuardCode, out)

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

// SteamLogStream streams live steamcmd login output via Server-Sent Events.
// It mirrors StreamLogs but targets the global sentinel login stream instead of
// a per-server log, so it takes no server ID. Clients should open this stream
// before POSTing to /steam/login to avoid missing the first lines.
func (r *Router) SteamLogStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	clientID, ch := r.streams.Subscribe(steamLogStreamID, logger.KindSteamCMD)
	defer r.streams.Unsubscribe(steamLogStreamID, logger.KindSteamCMD, clientID)

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
