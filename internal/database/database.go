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
// NOT drop columns — residual deprecated columns on older databases (e.g. the
// former port/query_port/rcon_port/rcon_enabled and the unused status column)
// are harmless because nothing reads them at runtime.
func migrate(db *gorm.DB) error {
	if err := dropLegacyModsIfEmpty(db); err != nil {
		return err
	}
	return db.AutoMigrate(
		&models.Server{},
		&models.Mod{},
		&models.User{},
	)
}

// dropLegacyModsIfEmpty removes an existing `mods` table only when it is empty,
// so AutoMigrate can recreate it cleanly.
//
// The legacy hand-written schema created `mods` with a raw
// `FOREIGN KEY (server_id) REFERENCES servers(id)` clause. When AutoMigrate
// needs to rebuild that table, the glebarez SQLite migrator mis-parses the DDL
// and treats the `FOREIGN` keyword as a column, crashing startup with
// "table mods__temp has no column named FOREIGN". Mod features are still stubs
// and the table is never populated, so dropping an EMPTY legacy table is a
// no-op data-wise and lets GORM recreate it without the problematic FK clause.
// A non-empty table is never dropped (defensive: no silent data loss).
func dropLegacyModsIfEmpty(db *gorm.DB) error {
	if !db.Migrator().HasTable("mods") {
		return nil
	}
	var count int64
	if err := db.Table("mods").Count(&count).Error; err != nil {
		return fmt.Errorf("count mods before legacy drop: %w", err)
	}
	if count > 0 {
		return nil // never drop data
	}
	if err := db.Migrator().DropTable("mods"); err != nil {
		return fmt.Errorf("drop legacy mods table: %w", err)
	}
	return nil
}
