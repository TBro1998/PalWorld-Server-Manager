package backup

import (
	"fmt"
	"os"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"gorm.io/gorm"
)

// Reconcile brings the backups table into agreement with the on-disk zips at
// startup. It only prunes DB rows whose zip has gone missing (deleted out of
// band): those rows would otherwise show phantom backups that fail to download
// or restore. Zips present on disk without a DB row are left untouched (they may
// be hand-placed archives without a manifest we recognize; we do not auto-adopt
// them to avoid ingesting arbitrary files).
//
// It never deletes zip files. Returns the number of orphan rows removed.
func Reconcile(db *gorm.DB) (int, error) {
	var rows []models.Backup
	if err := db.Find(&rows).Error; err != nil {
		return 0, err
	}

	removed := 0
	for _, r := range rows {
		// A still-"pending" path means a create crashed between row insert and
		// zip write — no file was ever finalized, so the row is an orphan.
		missing := r.FilePath == "" || r.FilePath == "pending"
		if !missing {
			if _, err := os.Stat(r.FilePath); err != nil && os.IsNotExist(err) {
				missing = true
			}
		}
		if missing {
			if err := db.Delete(&models.Backup{}, r.ID).Error; err != nil {
				return removed, fmt.Errorf("remove orphan backup %d: %w", r.ID, err)
			}
			removed++
		}
	}
	return removed, nil
}
