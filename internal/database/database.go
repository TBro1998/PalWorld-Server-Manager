package database

import (
	"fmt"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Initialize opens the SQLite database with the pure-Go glebarez driver (no CGO)
// and applies schema migrations via GORM AutoMigrate.
func Initialize(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// migrate applies the schema using GORM AutoMigrate. AutoMigrate is additive and
// idempotent: it creates missing tables/columns/indexes and is safe to run on
// every startup against both new and existing databases. It intentionally does
// NOT drop columns — residual deprecated columns on older databases are harmless
// because nothing reads them at runtime.
func migrate(db *gorm.DB) error {
	// Collect legacy per-server mod data before any table drops so we can
	// re-insert it into the new global-library schema.
	legacyMods, err := collectLegacyMods(db)
	if err != nil {
		return err
	}

	// Drop the mods table when it needs to be recreated under a new schema.
	// This is safe because either the table is empty, or we already read the
	// data above and will re-insert it after AutoMigrate.
	if err := dropModsIfNeeded(db); err != nil {
		return err
	}

	if err := db.AutoMigrate(
		&models.Server{},
		&models.Mod{},
		&models.ServerMod{},
		&models.User{},
		&models.Setting{},
	); err != nil {
		return err
	}

	// Re-insert legacy mod data into the new global-library + server_mods schema.
	return insertLegacyMods(db, legacyMods)
}

// legacyModRow mirrors the old per-server mods schema so we can read rows
// before dropping the table. All columns are read as strings to avoid type
// mismatch if GORM's column inference differs from the raw DDL.
type legacyModRow struct {
	ID          int64
	ServerID    int64
	WorkshopID  string
	Name        string
	Enabled     bool
	InstallPath string
	PackageName string
	ModName     string
	Version     string
}

// collectLegacyMods reads existing per-server mod rows from the mods table
// when it still has the old schema (server_id column present). Returns an empty
// slice when the table does not exist or already has the new schema.
func collectLegacyMods(db *gorm.DB) ([]legacyModRow, error) {
	if !db.Migrator().HasTable("mods") {
		return nil, nil
	}
	if !hasRawColumn(db, "mods", "server_id") {
		return nil, nil // already new schema
	}

	var rows []legacyModRow
	if err := db.Raw(`
		SELECT id, server_id, workshop_id, name, enabled, install_path,
		       package_name, mod_name, version
		FROM mods
	`).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("read legacy mods: %w", err)
	}
	return rows, nil
}

// dropModsIfNeeded drops the mods table when it must be recreated:
//   - Old schema (server_id column present): data was already read by collectLegacyMods.
//   - Legacy empty table (the original dropLegacyModsIfEmpty case): safe to drop.
//
// If the table has neither issue it is left untouched so AutoMigrate can add
// any new columns incrementally.
func dropModsIfNeeded(db *gorm.DB) error {
	if !db.Migrator().HasTable("mods") {
		return nil
	}

	// Old per-server schema: always drop so AutoMigrate can recreate cleanly.
	if hasRawColumn(db, "mods", "server_id") {
		if err := db.Migrator().DropTable("mods"); err != nil {
			return fmt.Errorf("drop legacy mods table: %w", err)
		}
		return nil
	}

	// Table exists with new schema — leave it for AutoMigrate to patch.
	return nil
}

// insertLegacyMods migrates legacy per-server mod rows into the new two-table
// schema (global mods library + server_mods junction). It deduplicates by
// WorkshopID so multiple servers sharing the same mod produce one global entry.
func insertLegacyMods(db *gorm.DB, rows []legacyModRow) error {
	if len(rows) == 0 {
		return nil
	}

	// Deduplicate by WorkshopID → global Mod entries.
	seen := make(map[string]int64) // workshopID → new global mod ID
	for _, r := range rows {
		if _, ok := seen[r.WorkshopID]; ok {
			continue
		}
		downloaded := r.InstallPath != ""
		mod := models.Mod{
			WorkshopID:   r.WorkshopID,
			Name:         r.Name,
			Downloaded:   downloaded,
			DownloadPath: r.InstallPath,
			PackageName:  r.PackageName,
			ModName:      r.ModName,
			Version:      r.Version,
		}
		if err := db.Create(&mod).Error; err != nil {
			return fmt.Errorf("insert legacy global mod %s: %w", r.WorkshopID, err)
		}
		seen[r.WorkshopID] = mod.ID
	}

	// Create server_mods entries that preserve the per-server enabled state.
	for _, r := range rows {
		modID, ok := seen[r.WorkshopID]
		if !ok {
			continue
		}
		sm := models.ServerMod{
			ServerID:        r.ServerID,
			ModID:           modID,
			Enabled:         r.Enabled,
			DeployedVersion: r.Version, // assume deployed = library version
		}
		if err := db.Create(&sm).Error; err != nil {
			return fmt.Errorf("insert legacy server_mod server=%d mod=%s: %w", r.ServerID, r.WorkshopID, err)
		}
	}
	return nil
}

// hasRawColumn reports whether table has a column with the given name by
// querying SQLite's pragma_table_info directly. This works even when the GORM
// model struct no longer declares that column (unlike db.Migrator().HasColumn).
func hasRawColumn(db *gorm.DB, table, column string) bool {
	var count int64
	db.Raw("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&count)
	return count > 0
}
