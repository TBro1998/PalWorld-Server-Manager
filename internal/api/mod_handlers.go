package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/logger"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palmod"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ── Global mod library ────────────────────────────────────────────────────────

// ModWithStatus extends models.Mod with runtime-computed fields for the UI.
type ModWithStatus struct {
	models.Mod
	// ServerCount is how many servers currently reference this mod.
	ServerCount int `json:"server_count"`
	// Downloading reports whether a SteamCMD download for this mod is currently
	// in progress. It is runtime state (not persisted), so a page refresh can
	// re-derive the in-flight download and reattach its log stream.
	Downloading bool `json:"downloading"`
}

// ListGlobalMods returns all mods in the global library, ordered by creation
// time. Each entry includes a server_count so the UI can indicate if a mod is
// in use before deletion.
// ListGlobalMods godoc
// @Summary      List global mods
// @Description  Returns all mods in the global library with server reference counts
// @Tags         mods
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /mods [get]
func (r *Router) ListGlobalMods(c *gin.Context) {
	var mods []models.Mod
	if err := r.db.Order("created_at DESC").Find(&mods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query mods"})
		return
	}
	if mods == nil {
		mods = []models.Mod{}
	}

	// Count server references per mod in one query.
	type countRow struct {
		ModID int64
		Count int
	}
	var counts []countRow
	r.db.Model(&models.ServerMod{}).
		Select("mod_id, COUNT(*) as count").
		Group("mod_id").
		Scan(&counts)
	countMap := make(map[int64]int, len(counts))
	for _, cr := range counts {
		countMap[cr.ModID] = cr.Count
	}

	result := make([]ModWithStatus, len(mods))
	for i, m := range mods {
		result[i] = ModWithStatus{
			Mod:         m,
			ServerCount: countMap[m.ID],
			Downloading: r.process.IsDownloadingGlobalMod(m.WorkshopID),
		}
	}
	c.JSON(http.StatusOK, gin.H{"mods": result})
}

// AddGlobalModRequest is the body for POST /api/mods.
type AddGlobalModRequest struct {
	WorkshopID string `json:"workshopId" binding:"required"`
	Name       string `json:"name"`
}

// AddGlobalMod godoc
// @Summary      Add mod to global library
// @Description  Registers a Steam Workshop mod in the global library (does not download)
// @Tags         mods
// @Accept       json
// @Produce      json
// @Param        body  body      AddGlobalModRequest  true  "workshopId and optional name"
// @Success      201   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      409   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /mods [post]
func (r *Router) AddGlobalMod(c *gin.Context) {
	var req AddGlobalModRequest
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

	// Upsert: if the mod already exists, return it without error so callers
	// (e.g. WorkshopBrowserDialog) can be idempotent.
	var existing models.Mod
	err := r.db.Where("workshop_id = ?", req.WorkshopID).First(&existing).Error
	if err == nil {
		c.JSON(http.StatusOK, existing)
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query mod"})
		return
	}

	mod := models.Mod{
		WorkshopID: req.WorkshopID,
		Name:       name,
		Downloaded: false,
	}
	if err := r.db.Create(&mod).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create mod"})
		return
	}
	c.JSON(http.StatusCreated, mod)
}

// DeleteGlobalMod removes a mod from the library. All server references
// (server_mods rows) are deleted first, and deployed content is removed from
// each server's Mods/Workshop directory.
// DeleteGlobalMod godoc
// @Summary      Delete mod from global library
// @Tags         mods
// @Produce      json
// @Param        modId  path      int  true  "Mod ID"
// @Success      200    {object}  map[string]interface{}
// @Failure      400    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /mods/{modId} [delete]
func (r *Router) DeleteGlobalMod(c *gin.Context) {
	modID, err := strconv.ParseInt(c.Param("modId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mod ID"})
		return
	}

	var mod models.Mod
	if err := r.db.First(&mod, modID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mod not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query mod"})
		return
	}

	// Remove deployed content from every server that references this mod.
	var serverMods []models.ServerMod
	r.db.Where("mod_id = ?", modID).Find(&serverMods)
	for _, sm := range serverMods {
		var srv models.Server
		if err := r.db.Select("install_path").First(&srv, sm.ServerID).Error; err == nil && srv.InstallPath != "" {
			if err := palmod.Remove(srv.InstallPath, mod.WorkshopID); err != nil {
				fmt.Printf("warning: remove mod %s from server %d: %v\n", mod.WorkshopID, sm.ServerID, err)
			}
			// Rewrite PalModSettings.ini for this server.
			if err := r.process.RewriteModSettings(sm.ServerID); err != nil {
				fmt.Printf("warning: rewrite PalModSettings.ini for server %d: %v\n", sm.ServerID, err)
			}
		}
	}

	// Delete all server references, then the global mod row.
	if err := r.db.Where("mod_id = ?", modID).Delete(&models.ServerMod{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete server references"})
		return
	}
	if err := r.db.Delete(&models.Mod{}, modID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete mod"})
		return
	}
	c.Status(http.StatusNoContent)
}

// DownloadGlobalMod triggers an async SteamCMD download for the given mod.
// Progress is streamed via GET /api/mods/:modId/logs/stream, keyed by the mod's
// own ID so that concurrent downloads have independent log channels.
// DownloadGlobalMod godoc
// @Summary      Download mod files via SteamCMD
// @Tags         mods
// @Produce      json
// @Param        modId  path      int  true  "Mod ID"
// @Success      200    {object}  map[string]interface{}
// @Failure      400    {object}  map[string]interface{}
// @Failure      404    {object}  map[string]interface{}
// @Failure      409    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /mods/{modId}/download [post]
func (r *Router) DownloadGlobalMod(c *gin.Context) {
	modID, err := strconv.ParseInt(c.Param("modId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mod ID"})
		return
	}

	var mod models.Mod
	if err := r.db.First(&mod, modID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mod not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query mod"})
		return
	}

	if r.process.IsDownloadingGlobalMod(mod.WorkshopID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mod is already downloading"})
		return
	}

	if err := logger.ResetLog(r.config.LogDir, modID, logger.KindSteamCMD); err != nil {
		fmt.Printf("warning: reset mod %d log: %v\n", modID, err)
	}

	go func() {
		capture := logger.NewCapture(modID, logger.KindSteamCMD, r.config.LogDir)
		broadcaster := logger.NewBroadcastWriter(r.streams, modID, logger.KindSteamCMD)
		out := io.MultiWriter(capture, broadcaster)
		defer capture.Close()

		result := "ok"
		if err := r.process.DownloadGlobalMod(modID, out); err != nil {
			result = "error"
			fmt.Printf("global mod %d download failed: %v\n", modID, err)
		}
		r.streams.BroadcastEvent(modID, logger.KindSteamCMD, "done", result)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Mod download started",
		"modId":   modID,
		"status":  "downloading",
	})
}

// GetModLogs returns the most recent captured download log lines for a single
// mod. The mod's download log is captured to disk under the SteamCMD log kind
// keyed by the mod's own ID, so this backfills history when the UI (re)attaches
// to an in-progress or finished download — mirroring GetLogs for servers.
// GetModLogs godoc
// @Summary      Get mod download logs
// @Description  Returns the most recent SteamCMD download log lines for a mod
// @Tags         mods
// @Produce      json
// @Param        modId  path      int  true   "Mod ID"
// @Param        lines  query     int  false  "Number of trailing lines (default 200)"
// @Success      200    {object}  map[string]interface{}
// @Failure      400    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /mods/{modId}/logs [get]
func (r *Router) GetModLogs(c *gin.Context) {
	modID, err := strconv.ParseInt(c.Param("modId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mod ID"})
		return
	}

	lines := 200
	if v := c.Query("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lines = n
		}
	}

	// Mod downloads only use the SteamCMD log kind; no kind query to parse.
	logs, err := logger.ReadLogs(r.config.LogDir, modID, logger.KindSteamCMD, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"modId": modID,
		"logs":  logs,
	})
}

// ModLogStream streams download progress for a single mod via SSE.
// Each mod uses its own ID as the stream key, allowing concurrent downloads
// to deliver logs independently without interleaving.
// ModLogStream godoc
// @Summary      Stream mod download logs (SSE)
// @Tags         mods
// @Produce      text/event-stream
// @Param        modId  path      int     true  "Mod ID"
// @Param        token  query     string  false "JWT token"
// @Success      200    {string}  string  "SSE stream"
// @Security     BearerAuth
// @Router       /mods/{modId}/logs/stream [get]
func (r *Router) ModLogStream(c *gin.Context) {
	modID, err := strconv.ParseInt(c.Param("modId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mod ID"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	clientID, ch := r.streams.Subscribe(modID, logger.KindSteamCMD)
	defer r.streams.Unsubscribe(modID, logger.KindSteamCMD, clientID)

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

// ── Server mod references ─────────────────────────────────────────────────────

// ServerModDetail is returned by ListServerMods. It embeds the ServerMod
// junction row plus the associated global Mod metadata and a computed
// VersionMismatch flag.
type ServerModDetail struct {
	models.ServerMod
	// Global mod metadata (flattened for convenience).
	WorkshopID   string   `json:"workshop_id"`
	Name         string   `json:"name"`
	ModName      string   `json:"mod_name"`
	PackageName  string   `json:"package_name"`
	Version      string   `json:"version"`
	Tags         []string `json:"tags"`
	Downloaded   bool     `json:"downloaded"`
	// VersionMismatch is true when the global library has a newer version than
	// what is currently deployed to this server.
	VersionMismatch bool `json:"version_mismatch"`
}

// ListServerMods returns all mods linked to a server, joined with global mod
// metadata and annotated with a version-mismatch flag.
// ListServerMods godoc
// @Summary      List server mods
// @Tags         mods
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/mods [get]
func (r *Router) ListServerMods(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	if !r.serverExists(c, id) {
		return
	}

	var serverMods []models.ServerMod
	if err := r.db.Where("server_id = ?", id).Order("created_at ASC").Find(&serverMods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server mods"})
		return
	}

	if len(serverMods) == 0 {
		c.JSON(http.StatusOK, gin.H{"mods": []ServerModDetail{}})
		return
	}

	// Batch-load the referenced global mods.
	modIDs := make([]int64, len(serverMods))
	for i, sm := range serverMods {
		modIDs[i] = sm.ModID
	}
	var globalMods []models.Mod
	if err := r.db.Where("id IN ?", modIDs).Find(&globalMods).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query global mods"})
		return
	}
	modMap := make(map[int64]models.Mod, len(globalMods))
	for _, m := range globalMods {
		modMap[m.ID] = m
	}

	result := make([]ServerModDetail, len(serverMods))
	for i, sm := range serverMods {
		gm := modMap[sm.ModID]
		mismatch := gm.Downloaded && gm.Version != "" && gm.Version != sm.DeployedVersion
		result[i] = ServerModDetail{
			ServerMod:       sm,
			WorkshopID:      gm.WorkshopID,
			Name:            gm.Name,
			ModName:         gm.ModName,
			PackageName:     gm.PackageName,
			Version:         gm.Version,
			Tags:            gm.Tags,
			Downloaded:      gm.Downloaded,
			VersionMismatch: mismatch,
		}
	}
	c.JSON(http.StatusOK, gin.H{"mods": result})
}

// LinkServerModRequest is the body for POST /api/servers/:id/mods.
type LinkServerModRequest struct {
	// ModID references an existing global library mod (preferred).
	ModID int64 `json:"modId"`
	// WorkshopID can be used instead of ModID; the mod must already be in the library.
	WorkshopID string `json:"workshopId"`
}

// LinkServerMod links an existing global library mod to a server.
// The mod must already be registered in the global library (AddGlobalMod first).
// LinkServerMod godoc
// @Summary      Link mod to server
// @Tags         mods
// @Accept       json
// @Produce      json
// @Param        id    path      int                   true  "Server ID"
// @Param        body  body      map[string]interface{}  true  "modId"
// @Success      201   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      409   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/mods [post]
func (r *Router) LinkServerMod(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	if !r.serverExists(c, id) {
		return
	}

	var req LinkServerModRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Resolve the global mod: by ID or by WorkshopID.
	var mod models.Mod
	if req.ModID != 0 {
		if err := r.db.First(&mod, req.ModID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Mod not found in library"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query mod"})
			return
		}
	} else if ws := strings.TrimSpace(req.WorkshopID); ws != "" {
		if err := r.db.Where("workshop_id = ?", ws).First(&mod).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Mod not found in library; add it first"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query mod"})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "modId or workshopId is required"})
		return
	}

	// Idempotent: if already linked, return existing entry.
	var existing models.ServerMod
	err = r.db.Where("server_id = ? AND mod_id = ?", id, mod.ID).First(&existing).Error
	if err == nil {
		c.JSON(http.StatusOK, existing)
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing link"})
		return
	}

	sm := models.ServerMod{
		ServerID: id,
		ModID:    mod.ID,
		Enabled:  true,
	}
	if err := r.db.Create(&sm).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link mod to server"})
		return
	}
	c.JSON(http.StatusCreated, sm)
}

// UnlinkServerMod removes a mod reference from a server: deletes the
// server_mods row, removes deployed content, and rewrites PalModSettings.ini.
// UnlinkServerMod godoc
// @Summary      Unlink mod from server
// @Tags         mods
// @Produce      json
// @Param        id           path      int  true  "Server ID"
// @Param        serverModId  path      int  true  "Server Mod ID"
// @Success      200          {object}  map[string]interface{}
// @Failure      400          {object}  map[string]interface{}
// @Failure      404          {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/mods/{serverModId} [delete]
func (r *Router) UnlinkServerMod(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	serverModID, err := strconv.ParseInt(c.Param("serverModId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server mod ID"})
		return
	}

	var sm models.ServerMod
	if err := r.db.First(&sm, serverModID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server mod not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server mod"})
		return
	}
	if sm.ServerID != id {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server mod not found"})
		return
	}

	// Look up global mod to get WorkshopID for directory removal.
	var mod models.Mod
	if err := r.db.First(&mod, sm.ModID).Error; err == nil {
		var srv models.Server
		if err := r.db.Select("install_path").First(&srv, id).Error; err == nil && srv.InstallPath != "" {
			if err := palmod.Remove(srv.InstallPath, mod.WorkshopID); err != nil {
				fmt.Printf("warning: remove mod %s from server %d: %v\n", mod.WorkshopID, id, err)
			}
		}
	}

	if err := r.db.Delete(&models.ServerMod{}, serverModID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlink mod"})
		return
	}

	// Rewrite the load config (best-effort).
	if err := r.process.RewriteModSettings(id); err != nil {
		fmt.Printf("warning: rewrite PalModSettings.ini for server %d: %v\n", id, err)
	}
	c.Status(http.StatusNoContent)
}

// ToggleServerMod flips a server mod's enabled flag, syncs the mod files on
// disk (remove when disabling, re-deploy from the global library when enabling),
// and rewrites PalModSettings.ini.
// ToggleServerMod godoc
// @Summary      Enable/disable server mod
// @Tags         mods
// @Accept       json
// @Produce      json
// @Param        id           path      int                   true  "Server ID"
// @Param        serverModId  path      int                   true  "Server Mod ID"
// @Param        body         body      map[string]interface{}  true  "enabled: true/false"
// @Success      200          {object}  map[string]interface{}
// @Failure      400          {object}  map[string]interface{}
// @Failure      404          {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/mods/{serverModId}/toggle [put]
func (r *Router) ToggleServerMod(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	serverModID, err := strconv.ParseInt(c.Param("serverModId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server mod ID"})
		return
	}

	var sm models.ServerMod
	if err := r.db.First(&sm, serverModID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Server mod not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query server mod"})
		return
	}
	if sm.ServerID != id {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server mod not found"})
		return
	}

	newEnabled := !sm.Enabled
	if err := r.db.Model(&models.ServerMod{}).Where("id = ?", serverModID).
		Update("enabled", newEnabled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update mod"})
		return
	}
	sm.Enabled = newEnabled

	// Sync files on disk to match the new enabled state.
	var mod models.Mod
	var srv models.Server
	hasMod := r.db.First(&mod, sm.ModID).Error == nil
	hasSrv := hasMod &&
		r.db.Select("install_path").First(&srv, id).Error == nil &&
		srv.InstallPath != ""

	if hasSrv {
		if !newEnabled {
			// Disabling: remove deployed mod files so the server directory stays clean.
			if err := palmod.Remove(srv.InstallPath, mod.WorkshopID); err != nil {
				fmt.Printf("warning: remove mod %s from server %d: %v\n", mod.WorkshopID, id, err)
			}
			// Clear deployed_version — files are no longer present.
			if err := r.db.Model(&models.ServerMod{}).Where("id = ?", serverModID).
				Update("deployed_version", "").Error; err != nil {
				fmt.Printf("warning: clear deployed_version for server_mod %d: %v\n", serverModID, err)
			}
			sm.DeployedVersion = ""
		} else if mod.Downloaded && mod.DownloadPath != "" {
			// Enabling: re-deploy mod files from the global library staging area.
			if _, err := palmod.Deploy(srv.InstallPath, mod.WorkshopID, mod.DownloadPath); err != nil {
				fmt.Printf("warning: re-deploy mod %s to server %d: %v\n", mod.WorkshopID, id, err)
			} else {
				// Update deployed_version to match the current global library version.
				if err := r.db.Model(&models.ServerMod{}).Where("id = ?", serverModID).
					Update("deployed_version", mod.Version).Error; err != nil {
					fmt.Printf("warning: update deployed_version for server_mod %d: %v\n", serverModID, err)
				}
				sm.DeployedVersion = mod.Version
			}
		}
	}

	// Rewrite PalModSettings.ini to reflect the toggle.
	if err := r.process.RewriteModSettings(id); err != nil {
		fmt.Printf("warning: rewrite PalModSettings.ini for server %d: %v\n", id, err)
	}
	c.JSON(http.StatusOK, sm)
}

// DeployServerMods copies all enabled global mods into the server's
// Mods/Workshop directory, updates deployed_version on each server_mod row,
// and rewrites PalModSettings.ini. Progress is streamed via the server's
// existing steamcmd log stream (KindSteamCMD).
// DeployServerMods godoc
// @Summary      Deploy mods to game directory
// @Tags         mods
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/mods/deploy [post]
func (r *Router) DeployServerMods(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	if !r.serverExists(c, id) {
		return
	}

	if r.process.IsInstalling(id) || r.process.IsDeployingMods(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server is installing or mods are already deploying"})
		return
	}

	if err := logger.ResetLog(r.config.LogDir, id, logger.KindSteamCMD); err != nil {
		fmt.Printf("warning: reset steamcmd log for server %d: %v\n", id, err)
	}

	go func() {
		capture := logger.NewCapture(id, logger.KindSteamCMD, r.config.LogDir)
		broadcaster := logger.NewBroadcastWriter(r.streams, id, logger.KindSteamCMD)
		out := io.MultiWriter(capture, broadcaster)
		defer capture.Close()

		result := "ok"
		if err := r.process.DeployServerMods(id, out); err != nil {
			result = "error"
			fmt.Printf("server %d mod deploy failed: %v\n", id, err)
		}
		r.streams.BroadcastEvent(id, logger.KindSteamCMD, "done", result)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":  "Mod deployment started",
		"serverId": id,
		"status":   "deploying",
	})
}
