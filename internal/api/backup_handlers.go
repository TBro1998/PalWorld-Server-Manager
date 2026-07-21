package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// backupDTO is the client-facing view of a backup. FilePath is intentionally
// omitted (internal detail).
type backupDTO struct {
	ID        int64     `json:"id"`
	ServerID  int64     `json:"server_id"`
	Scope     string    `json:"scope"`
	Source    string    `json:"source"`
	Hot       bool      `json:"hot"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

func toBackupDTO(b models.Backup) backupDTO {
	return backupDTO{
		ID:        b.ID,
		ServerID:  b.ServerID,
		Scope:     b.Scope,
		Source:    b.Source,
		Hot:       b.Hot,
		SizeBytes: b.SizeBytes,
		CreatedAt: b.CreatedAt,
	}
}

// backupServerID parses :id, writing a 400 on failure. Returns (id, ok).
func backupServerID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return 0, false
	}
	return id, true
}

// backupID parses :backupId, writing a 400 on failure. Returns (id, ok).
func backupID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("backupId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return 0, false
	}
	return id, true
}

// ListBackups returns all backups for a server, newest first.
// ListBackups godoc
// @Summary      List server backups
// @Description  Returns all backups for a server ordered newest first
// @Tags         backup
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/backups [get]
func (r *Router) ListBackups(c *gin.Context) {
	id, ok := backupServerID(c)
	if !ok {
		return
	}
	rows, err := r.backups.List(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list backups"})
		return
	}
	out := make([]backupDTO, 0, len(rows))
	for _, b := range rows {
		out = append(out, toBackupDTO(b))
	}
	c.JSON(http.StatusOK, gin.H{"backups": out})
}

type createBackupRequest struct {
	// Scope: "save" | "config" | "all". Defaults to "all" when omitted.
	Scope string `json:"scope"`
}

// CreateBackup creates a manual backup of the given scope.
// CreateBackup godoc
// @Summary      Create a backup
// @Description  Creates a manual backup of the server's save and/or config
// @Tags         backup
// @Accept       json
// @Produce      json
// @Param        id    path      int                     true  "Server ID"
// @Param        body  body      map[string]interface{}  false "scope: save|config|all"
// @Success      201   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/backups [post]
func (r *Router) CreateBackup(c *gin.Context) {
	id, ok := backupServerID(c)
	if !ok {
		return
	}
	var req createBackupRequest
	// Body is optional; ignore bind errors and fall back to default scope.
	_ = c.ShouldBindJSON(&req)
	scope := req.Scope
	if scope == "" {
		scope = models.BackupScopeAll
	}

	rec, err := r.backups.Create(id, scope, models.BackupSourceManual, time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, toBackupDTO(*rec))
}

// DownloadBackup streams the backup zip as an attachment.
// DownloadBackup godoc
// @Summary      Download a backup
// @Description  Downloads the backup archive (zip)
// @Tags         backup
// @Produce      application/zip
// @Param        id        path      int  true  "Server ID"
// @Param        backupId  path      int  true  "Backup ID"
// @Success      200       {file}    binary
// @Failure      400       {object}  map[string]interface{}
// @Failure      404       {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/backups/{backupId}/download [get]
func (r *Router) DownloadBackup(c *gin.Context) {
	id, ok := backupServerID(c)
	if !ok {
		return
	}
	bid, ok := backupID(c)
	if !ok {
		return
	}
	rec, err := r.backups.Get(id, bid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load backup"})
		return
	}
	filename := "server-" + strconv.FormatInt(id, 10) + "-backup-" + strconv.FormatInt(bid, 10) + ".zip"
	c.FileAttachment(rec.FilePath, filename)
}

// DeleteBackup removes a backup (file + record).
// DeleteBackup godoc
// @Summary      Delete a backup
// @Description  Deletes the backup archive and its record
// @Tags         backup
// @Produce      json
// @Param        id        path      int  true  "Server ID"
// @Param        backupId  path      int  true  "Backup ID"
// @Success      200       {object}  map[string]interface{}
// @Failure      400       {object}  map[string]interface{}
// @Failure      500       {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/backups/{backupId} [delete]
func (r *Router) DeleteBackup(c *gin.Context) {
	id, ok := backupServerID(c)
	if !ok {
		return
	}
	bid, ok := backupID(c)
	if !ok {
		return
	}
	if err := r.backups.Delete(id, bid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// RestoreBackup restores a backup. The server must be stopped.
// RestoreBackup godoc
// @Summary      Restore a backup
// @Description  Restores the server save/config from a backup. Server must be stopped. A pre-restore safety backup is taken first.
// @Tags         backup
// @Produce      json
// @Param        id        path      int  true  "Server ID"
// @Param        backupId  path      int  true  "Backup ID"
// @Success      200       {object}  map[string]interface{}
// @Failure      400       {object}  map[string]interface{}
// @Failure      404       {object}  map[string]interface{}
// @Failure      409       {object}  map[string]interface{}
// @Failure      500       {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/backups/{backupId}/restore [post]
func (r *Router) RestoreBackup(c *gin.Context) {
	id, ok := backupServerID(c)
	if !ok {
		return
	}
	bid, ok := backupID(c)
	if !ok {
		return
	}
	// Restoring while the process holds the save files open is unsafe: require
	// a stopped server (mirrors the config-write "server must be stopped" rule).
	if r.process.IsRunning(id) {
		c.JSON(http.StatusConflict, gin.H{"error": "Server must be stopped before restoring a backup"})
		return
	}
	if err := r.backups.Restore(id, bid, time.Now()); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "restored"})
}

// scheduleDTO is the client view of a server's backup schedule.
type scheduleDTO struct {
	ServerID        int64  `json:"server_id"`
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"interval_minutes"`
	Scope           string `json:"scope"`
	KeepCount       int    `json:"keep_count"`
	KeepDays        int    `json:"keep_days"`
}

func toScheduleDTO(s models.BackupSchedule) scheduleDTO {
	return scheduleDTO{
		ServerID:        s.ServerID,
		Enabled:         s.Enabled,
		IntervalMinutes: s.IntervalMinutes,
		Scope:           s.Scope,
		KeepCount:       s.KeepCount,
		KeepDays:        s.KeepDays,
	}
}

// GetBackupSchedule returns the server's automatic-backup config, or sensible
// defaults when none is set yet.
// GetBackupSchedule godoc
// @Summary      Get backup schedule
// @Description  Returns the automatic-backup configuration for a server
// @Tags         backup
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/backups/schedule [get]
func (r *Router) GetBackupSchedule(c *gin.Context) {
	id, ok := backupServerID(c)
	if !ok {
		return
	}
	var sched models.BackupSchedule
	err := r.db.First(&sched, "server_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Not configured yet — return defaults so the form has values.
		c.JSON(http.StatusOK, scheduleDTO{
			ServerID:        id,
			Enabled:         false,
			IntervalMinutes: 60,
			Scope:           models.BackupScopeAll,
			KeepCount:       10,
			KeepDays:        0,
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load schedule"})
		return
	}
	c.JSON(http.StatusOK, toScheduleDTO(sched))
}

type updateScheduleRequest struct {
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"interval_minutes"`
	Scope           string `json:"scope"`
	KeepCount       int    `json:"keep_count"`
	KeepDays        int    `json:"keep_days"`
}

// UpdateBackupSchedule upserts the server's automatic-backup config and reloads
// the scheduler for that server.
// UpdateBackupSchedule godoc
// @Summary      Update backup schedule
// @Description  Sets the automatic-backup configuration for a server
// @Tags         backup
// @Accept       json
// @Produce      json
// @Param        id    path      int                     true  "Server ID"
// @Param        body  body      map[string]interface{}  true  "schedule fields"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/backups/schedule [put]
func (r *Router) UpdateBackupSchedule(c *gin.Context) {
	id, ok := backupServerID(c)
	if !ok {
		return
	}
	var req updateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Validate inputs.
	if req.Enabled && req.IntervalMinutes <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "interval_minutes must be positive when enabled"})
		return
	}
	scope := req.Scope
	if scope == "" {
		scope = models.BackupScopeAll
	}
	switch scope {
	case models.BackupScopeSave, models.BackupScopeConfig, models.BackupScopeAll:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scope"})
		return
	}
	if req.KeepCount < 0 || req.KeepDays < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "keep_count/keep_days must be >= 0"})
		return
	}

	sched := models.BackupSchedule{
		ServerID:        id,
		Enabled:         req.Enabled,
		IntervalMinutes: req.IntervalMinutes,
		Scope:           scope,
		KeepCount:       req.KeepCount,
		KeepDays:        req.KeepDays,
	}
	// Upsert on the primary key (server_id).
	if err := r.db.Save(&sched).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save schedule"})
		return
	}

	// Reflect the change in the running scheduler immediately.
	if r.backupScheduler != nil {
		r.backupScheduler.Reload(id)
	}
	c.JSON(http.StatusOK, toScheduleDTO(sched))
}
