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

// ApplyUpdate downloads the new binary, verifies its checksum, replaces the
// running executable via minio/selfupdate, and re-execs the process.
//
// Progress is streamed to SSE subscribers on /api/system/update/stream.
// This endpoint returns 202 Accepted immediately; the actual work runs async.
//
// SECURITY: This endpoint triggers binary replacement and process restart.
// Registered under the protected group; JWT auth will cover it once enabled.
func (r *Router) ApplyUpdate(c *gin.Context) {
	result := update.Cached()
	if result == nil || !result.HasUpdate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no update available; run check first"})
		return
	}

	mirror, _ := settings.Get(r.db, settings.KeyDownloadMirror)

	progress := func(pct int, msg string) {
		if pct >= 0 {
			r.streams.BroadcastEvent(update.UpdateStreamID, logger.KindUpdate, "progress",
				fmt.Sprintf(`{"pct":%d,"msg":%q}`, pct, msg))
		} else {
			r.streams.Broadcast(update.UpdateStreamID, logger.KindUpdate, msg)
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		if err := update.Apply(ctx, result, mirror, progress); err != nil {
			r.streams.BroadcastEvent(update.UpdateStreamID, logger.KindUpdate, "error",
				fmt.Sprintf(`{"error":%q}`, err.Error()))
		} else {
			// Apply only returns if restart itself failed; signal the client.
			r.streams.BroadcastEvent(update.UpdateStreamID, logger.KindUpdate, "restarting", `{}`)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "update started"})
}

// UpdateStream is an SSE endpoint that streams update progress events to the
// browser.  Open this EventSource before calling ApplyUpdate to catch all
// progress lines from the beginning.
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
