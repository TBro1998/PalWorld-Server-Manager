package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/settings"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/update"
	"github.com/gin-gonic/gin"
)

// GetVersion returns the running binary's build metadata.
// @Summary Get version info
// @Tags system
// @Produce json
// @Success 200 {object} map[string]interface{} "version, buildTime, gitCommit"
// @Router /system/version [get]
func (r *Router) GetVersion(c *gin.Context) {
	info := r.checker.BuildInfo()
	c.JSON(http.StatusOK, gin.H{
		"version":   info.Version,
		"buildTime": info.BuildTime,
		"gitCommit": info.GitCommit,
	})
}

// CheckUpdate queries GitHub for the latest release and compares it with the
// current version.  Pass ?cached=1 to return the in-memory cached result
// without hitting GitHub again.
// @Summary Check for updates
// @Tags system
// @Produce json
// @Param cached query string false "Return cached result (1 for cached)"
// @Success 200 {object} map[string]interface{} "update check result"
// @Security BearerAuth
// @Router /system/update/check [get]
func (r *Router) CheckUpdate(c *gin.Context) {
	if c.Query("cached") == "1" {
		if cached := update.Cached(); cached != nil {
			c.JSON(http.StatusOK, cached)
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	result, err := r.checker.Check(ctx)
	if err != nil {
		// Return the result even on error — it contains the Err field.
		c.JSON(http.StatusOK, result)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetUpdateStatus returns the current update phase so the UI can restore
// progress after a page navigation or browser refresh.
// @Summary Get current update status
// @Tags system
// @Produce json
// @Success 200 {object} map[string]interface{} "phase, pct, msg, err"
// @Security BearerAuth
// @Router /system/update/status [get]
func (r *Router) GetUpdateStatus(c *gin.Context) {
	c.JSON(http.StatusOK, update.Status())
}

// ApplyUpdate downloads the new binary, verifies its checksum, replaces the
// running executable via minio/selfupdate, and re-execs the process.
//
// Progress is streamed to SSE subscribers on /api/system/update/stream.
// This endpoint returns 202 Accepted immediately; the actual work runs async.
// Returns 409 Conflict if an update is already in progress.
//
// SECURITY: This endpoint triggers binary replacement and process restart.
// Registered under the protected group; JWT auth will cover it once enabled.
// @Summary Apply system update
// @Tags system
// @Produce json
// @Success 202 {object} map[string]interface{} "update started"
// @Failure 400 {object} map[string]interface{} "no update available"
// @Failure 409 {object} map[string]interface{} "update already in progress"
// @Security BearerAuth
// @Router /system/update/apply [post]
func (r *Router) ApplyUpdate(c *gin.Context) {
	result := update.Cached()
	if result == nil || !result.HasUpdate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no update available; run check first"})
		return
	}

	// Guard: refuse a second download if one is already running.
	if s := update.Status(); s.Phase == update.PhaseDownloading || s.Phase == update.PhaseRestarting {
		c.JSON(http.StatusConflict, gin.H{"error": "update already in progress"})
		return
	}

	mirror, _ := settings.Get(r.db, settings.KeyDownloadMirror)

	progress := func(pct int, msg string) {
		if pct >= 0 {
			// Keep status store in sync so late-joining clients see current progress.
			update.SetStatus(update.UpdateStatus{Phase: update.PhaseDownloading, Pct: pct, Msg: msg})
			r.streams.BroadcastEvent(update.UpdateStreamID, logger.KindUpdate, "progress",
				fmt.Sprintf(`{"pct":%d,"msg":%q}`, pct, msg))
		} else {
			r.streams.Broadcast(update.UpdateStreamID, logger.KindUpdate, msg)
		}
	}

	update.SetStatus(update.UpdateStatus{Phase: update.PhaseDownloading})

	// onRestarting is called from within Apply() right before os.Exit, giving
	// us a window to flush the "restarting" SSE event to clients while the
	// HTTP server is still alive.
	onRestarting := func() {
		update.SetStatus(update.UpdateStatus{Phase: update.PhaseRestarting})
		r.streams.BroadcastEvent(update.UpdateStreamID, logger.KindUpdate, "restarting", `{}`)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// Apply never returns on success — it calls os.Exit(0) after launching
		// the new process.  It only returns here when the download or binary
		// replacement step fails before any file has been replaced.
		if err := update.Apply(ctx, result, mirror, progress, onRestarting); err != nil {
			update.SetStatus(update.UpdateStatus{Phase: update.PhaseError, Err: err.Error()})
			r.streams.BroadcastEvent(update.UpdateStreamID, logger.KindUpdate, "error",
				fmt.Sprintf(`{"error":%q}`, err.Error()))
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "update started"})
}

// UpdateStream is an SSE endpoint that streams update progress events to the
// browser.  Open this EventSource before calling ApplyUpdate to catch all
// progress lines from the beginning.
// @Summary Stream update progress (SSE)
// @Tags system
// @Produce text/event-stream
// @Success 200 {string} string "SSE stream of update events"
// @Security BearerAuth
// @Router /system/update/stream [get]
func (r *Router) UpdateStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	clientID, ch := r.streams.Subscribe(update.UpdateStreamID, logger.KindUpdate)
	defer r.streams.Unsubscribe(update.UpdateStreamID, logger.KindUpdate, clientID)

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

// GetSystemSettings returns runtime-adjustable settings exposed to the UI.
// Currently exposes download_mirror only.
// @Summary Get system settings
// @Tags system
// @Produce json
// @Success 200 {object} map[string]interface{} "download_mirror"
// @Failure 500 {object} map[string]interface{} "failed to read settings"
// @Security BearerAuth
// @Router /system/settings [get]
func (r *Router) GetSystemSettings(c *gin.Context) {
	mirror, err := settings.Get(r.db, settings.KeyDownloadMirror)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"download_mirror": mirror,
	})
}

// UpdateSystemSettings persists UI-configurable system settings.
// @Summary Update system settings
// @Tags system
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "download_mirror"
// @Success 200 {object} map[string]interface{} "download_mirror"
// @Failure 400 {object} map[string]interface{} "invalid request"
// @Failure 500 {object} map[string]interface{} "failed to save settings"
// @Security BearerAuth
// @Router /system/settings [put]
func (r *Router) UpdateSystemSettings(c *gin.Context) {
	var body struct {
		DownloadMirror string `json:"download_mirror"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := settings.Set(r.db, settings.KeyDownloadMirror, body.DownloadMirror); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"download_mirror": body.DownloadMirror})
}
