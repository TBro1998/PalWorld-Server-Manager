package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/palconfig"
	"gorm.io/gorm"
)

// saveGamesRel is the world-save root relative to a server's install path. The
// whole SaveGames/0 tree is archived (not a single world dir) so multi-world
// installs and worldid drift are handled uniformly on restore.
var saveGamesRel = filepath.Join("Pal", "Saved", "SaveGames", "0")

// RunningFunc reports whether a server's process is currently running.
type RunningFunc func(serverID int64) bool

// SaveFunc best-effort triggers an in-game save (REST /save) before a hot
// backup. It must return quickly and its error is non-fatal (logged by caller).
type SaveFunc func(serverID int64) error

// Service owns backup creation/restore/pruning against the DB and the backup
// root directory. It is filesystem + DB glue; HTTP concerns stay in the API
// layer. Safe for concurrent use (stateless beyond its deps).
type Service struct {
	db        *gorm.DB
	backupDir string
	isRunning RunningFunc
	save      SaveFunc
}

// NewService constructs a backup Service. isRunning/save may be nil (treated as
// "never running" / "no-op save") to keep the package testable without the API
// layer.
func NewService(db *gorm.DB, backupDir string, isRunning RunningFunc, save SaveFunc) *Service {
	if isRunning == nil {
		isRunning = func(int64) bool { return false }
	}
	if save == nil {
		save = func(int64) error { return nil }
	}
	return &Service{db: db, backupDir: backupDir, isRunning: isRunning, save: save}
}

// serverBackupDir returns <backupDir>/<serverID>.
func (s *Service) serverBackupDir(serverID int64) string {
	return filepath.Join(s.backupDir, strconv.FormatInt(serverID, 10))
}

// zipPath returns the on-disk path for a backup id under a server dir.
func (s *Service) zipPath(serverID, backupID int64) string {
	return filepath.Join(s.serverBackupDir(serverID), strconv.FormatInt(backupID, 10)+".zip")
}

// sourceFor builds the archive Source for a scope given a server install path.
func sourceFor(scope, installPath string) Source {
	var src Source
	if scope == models.BackupScopeSave || scope == models.BackupScopeAll {
		src.SaveDir = filepath.Join(installPath, saveGamesRel)
	}
	if scope == models.BackupScopeConfig || scope == models.BackupScopeAll {
		src.ConfigDir = palconfig.ConfigDir(installPath)
	}
	return src
}

// validScope reports whether scope is one of the known values.
func validScope(scope string) bool {
	switch scope {
	case models.BackupScopeSave, models.BackupScopeConfig, models.BackupScopeAll:
		return true
	}
	return false
}

// Create archives the given scope for a server and records a Backup row. When
// the server is running it best-effort triggers an in-game save first and marks
// the backup hot. After a successful create it applies the server's retention
// policy. now is passed for deterministic timestamps in tests; pass time.Now().
func (s *Service) Create(serverID int64, scope, source string, now time.Time) (*models.Backup, error) {
	if !validScope(scope) {
		return nil, fmt.Errorf("invalid backup scope %q", scope)
	}

	var srv models.Server
	if err := s.db.Select("install_path").First(&srv, serverID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("server %d not found", serverID)
		}
		return nil, err
	}
	if srv.InstallPath == "" {
		return nil, fmt.Errorf("server %d has no install path", serverID)
	}

	hot := s.isRunning(serverID)
	if hot {
		// Best-effort flush to disk; ignore errors (REST may be disabled).
		_ = s.save(serverID)
	}

	// Create the row first so its auto-increment ID names the zip file.
	rec := &models.Backup{
		ServerID:  serverID,
		Scope:     scope,
		Source:    source,
		Hot:       hot,
		CreatedAt: now,
	}
	// FilePath is NOT NULL; set a placeholder we overwrite after we know the ID.
	rec.FilePath = "pending"
	if err := s.db.Create(rec).Error; err != nil {
		return nil, fmt.Errorf("create backup record: %w", err)
	}

	dest := s.zipPath(serverID, rec.ID)
	m := Manifest{
		ServerID:  serverID,
		Scope:     scope,
		Source:    source,
		Hot:       hot,
		CreatedAt: now,
	}
	if err := CreateZip(dest, sourceFor(scope, srv.InstallPath), m); err != nil {
		// Roll back the orphan row so the list never shows a phantom backup.
		s.db.Delete(&models.Backup{}, rec.ID)
		return nil, err
	}

	size := int64(0)
	if fi, statErr := os.Stat(dest); statErr == nil {
		size = fi.Size()
	}
	if err := s.db.Model(&models.Backup{}).Where("id = ?", rec.ID).
		Updates(map[string]any{"file_path": dest, "size_bytes": size}).Error; err != nil {
		return nil, fmt.Errorf("persist backup path: %w", err)
	}
	rec.FilePath = dest
	rec.SizeBytes = size

	// Apply retention (best-effort; a prune failure never fails the create).
	if err := s.applyRetention(serverID, now); err != nil {
		fmt.Printf("warning: backup retention for server %d: %v\n", serverID, err)
	}

	return rec, nil
}

// applyRetention prunes old backups for a server per its schedule policy. When
// no schedule row exists, nothing is pruned (0/0 = unlimited).
func (s *Service) applyRetention(serverID int64, now time.Time) error {
	var sched models.BackupSchedule
	err := s.db.First(&sched, "server_id = ?", serverID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if sched.KeepCount <= 0 && sched.KeepDays <= 0 {
		return nil
	}

	var rows []models.Backup
	if err := s.db.Where("server_id = ?", serverID).Find(&rows).Error; err != nil {
		return err
	}
	items := make([]RetentionItem, len(rows))
	for i, r := range rows {
		items[i] = RetentionItem{ID: r.ID, CreatedAt: r.CreatedAt}
	}
	for _, id := range Prune(items, sched.KeepCount, sched.KeepDays, now) {
		_ = s.Delete(serverID, id)
	}
	return nil
}

// Delete removes a backup's zip and DB row. Missing zip is not an error (the
// record is still removed). A backup id not owned by serverID is rejected.
func (s *Service) Delete(serverID, backupID int64) error {
	var rec models.Backup
	err := s.db.First(&rec, backupID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if rec.ServerID != serverID {
		return fmt.Errorf("backup %d does not belong to server %d", backupID, serverID)
	}
	if rec.FilePath != "" && rec.FilePath != "pending" {
		if err := os.Remove(rec.FilePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove backup file: %w", err)
		}
	}
	return s.db.Delete(&models.Backup{}, backupID).Error
}

// Get loads a single backup row, enforcing server ownership.
func (s *Service) Get(serverID, backupID int64) (*models.Backup, error) {
	var rec models.Backup
	if err := s.db.First(&rec, backupID).Error; err != nil {
		return nil, err
	}
	if rec.ServerID != serverID {
		return nil, gorm.ErrRecordNotFound
	}
	return &rec, nil
}

// List returns a server's backups, newest first.
func (s *Service) List(serverID int64) ([]models.Backup, error) {
	var rows []models.Backup
	if err := s.db.Where("server_id = ?", serverID).
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Restore replaces the server's on-disk save/config with the contents of the
// given backup. The caller must ensure the server is stopped. A pre-restore
// safety backup (scope=all) is taken first. now is used for the pre-restore
// timestamp.
func (s *Service) Restore(serverID, backupID int64, now time.Time) error {
	rec, err := s.Get(serverID, backupID)
	if err != nil {
		return err
	}

	var srv models.Server
	if err := s.db.Select("install_path").First(&srv, serverID).Error; err != nil {
		return err
	}
	if srv.InstallPath == "" {
		return fmt.Errorf("server %d has no install path", serverID)
	}

	// Safety snapshot before mutating anything on disk.
	if _, err := s.Create(serverID, models.BackupScopeAll, models.BackupSourcePreRestore, now); err != nil {
		return fmt.Errorf("pre-restore backup: %w", err)
	}

	targets := []RestoreTarget{
		{Prefix: savePrefix, DestDir: filepath.Join(srv.InstallPath, saveGamesRel)},
		{Prefix: configPrefix, DestDir: palconfig.ConfigDir(srv.InstallPath)},
	}
	if err := ExtractInto(rec.FilePath, targets); err != nil {
		return fmt.Errorf("restore backup %d: %w", backupID, err)
	}
	return nil
}
