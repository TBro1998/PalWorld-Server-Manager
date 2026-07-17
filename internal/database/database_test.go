package database

import (
	"path/filepath"
	"testing"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// closeDB releases the underlying sql.DB so Windows can remove the temp file
// during t.TempDir cleanup (an open handle blocks the unlink otherwise).
func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}

// TestInitializeFreshDB verifies AutoMigrate creates the schema and basic CRUD
// works, including that Create auto-populates ID and timestamps.
func TestInitializeFreshDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	db, err := Initialize(dbPath)
	if err != nil {
		t.Fatalf("Initialize fresh db: %v", err)
	}
	defer closeDB(t, db)

	s := models.Server{Name: "s1", InstallPath: "/tmp/s1"}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.ID == 0 {
		t.Fatal("expected auto-populated ID")
	}
	if s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		t.Fatal("expected auto-populated timestamps")
	}

	var got models.Server
	if err := db.First(&got, s.ID).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.Name != "s1" || got.PID != 0 || got.Installed {
		t.Fatalf("unexpected row: %+v", got)
	}
}

// TestZeroValueUpdatesPersist guards the map/single-column update convention:
// pid=0, last_error="" and installed=false must actually be written (a struct
// Updates would skip these zero values).
func TestZeroValueUpdatesPersist(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "zero.db")
	db, err := Initialize(dbPath)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	defer closeDB(t, db)

	s := models.Server{Name: "s", InstallPath: "/p", PID: 123, LastError: "boom", Installed: true}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	// Mirror manager.setPID / setError / installed reset.
	if err := db.Model(&models.Server{}).Where("id = ?", s.ID).Update("pid", 0).Error; err != nil {
		t.Fatalf("update pid: %v", err)
	}
	if err := db.Model(&models.Server{}).Where("id = ?", s.ID).Update("last_error", "").Error; err != nil {
		t.Fatalf("update last_error: %v", err)
	}
	if err := db.Model(&models.Server{}).Where("id = ?", s.ID).Update("installed", false).Error; err != nil {
		t.Fatalf("update installed: %v", err)
	}

	var got models.Server
	if err := db.First(&got, s.ID).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.PID != 0 || got.LastError != "" || got.Installed {
		t.Fatalf("zero-value update did not persist: %+v", got)
	}
}

// TestModTagsSerializerRoundTrip guards the mod_name/tags backfill: the Tags
// []string column uses gorm serializer:json, and the manager backfills it via
// Updates(map[string]any{...}) (mirrored here). This verifies the serializer is
// applied through the map-update path — a []string in, a []string out — and that
// an absent value reads back as nil (empty mod list case).
func TestModTagsSerializerRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mods.db")
	db, err := Initialize(dbPath)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	defer closeDB(t, db)

	srv := models.Server{Name: "s", InstallPath: "/p"}
	if err := db.Create(&srv).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}
	mod := models.Mod{ServerID: srv.ID, WorkshopID: "123", Name: "123", Enabled: true}
	if err := db.Create(&mod).Error; err != nil {
		t.Fatalf("create mod: %v", err)
	}
	// Fresh row: tags absent -> nil.
	var fresh models.Mod
	if err := db.First(&fresh, mod.ID).Error; err != nil {
		t.Fatalf("first fresh: %v", err)
	}
	if fresh.Tags != nil {
		t.Errorf("fresh Tags should be nil, got %#v", fresh.Tags)
	}

	// Backfill exactly as manager.UpdateMods does (Select + struct Updates so the
	// Tags serializer is applied and zero values are still written).
	if err := db.Model(&models.Mod{}).Where("id = ?", mod.ID).
		Select("package_name", "mod_name", "version", "tags", "install_path").
		Updates(models.Mod{
			PackageName: "MyMod",
			ModName:     "My Mod",
			Version:     "1.2.3",
			Tags:        []string{"Gameplay", "QoL"},
			InstallPath: "/p/Mods/Workshop/123",
		}).Error; err != nil {
		t.Fatalf("backfill: %v", err)
	}

	var got models.Mod
	if err := db.First(&got, mod.ID).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.ModName != "My Mod" || got.PackageName != "MyMod" || got.Version != "1.2.3" {
		t.Fatalf("unexpected scalar backfill: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "Gameplay" || got.Tags[1] != "QoL" {
		t.Fatalf("tags serializer round-trip failed: %#v", got.Tags)
	}
}

// TestInitializeLegacyDB verifies AutoMigrate opens an existing database that
// still carries deprecated columns (port/query_port/rcon_port/rcon_enabled) and
// the unused status column without error (AC4).
func TestInitializeLegacyDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	// Build a legacy-shaped servers table directly, then close.
	raw, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	legacy := `CREATE TABLE servers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		install_path TEXT NOT NULL,
		status TEXT DEFAULT 'stopped',
		port INTEGER DEFAULT 0,
		query_port INTEGER DEFAULT 0,
		rcon_port INTEGER DEFAULT 0,
		rcon_enabled BOOLEAN DEFAULT 0,
		pid INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if err := raw.Exec(legacy).Error; err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if err := raw.Exec(`INSERT INTO servers (name, install_path) VALUES ('old', '/old')`).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	if sqlDB, err := raw.DB(); err == nil {
		sqlDB.Close()
	}

	// Now run the real Initialize (AutoMigrate) over the legacy db.
	db, err := Initialize(dbPath)
	if err != nil {
		t.Fatalf("Initialize legacy db: %v", err)
	}
	defer closeDB(t, db)

	// Existing row is readable; new columns (launch_args/installed/last_error) added.
	var got models.Server
	if err := db.First(&got, 1).Error; err != nil {
		t.Fatalf("read legacy row: %v", err)
	}
	if got.Name != "old" || got.InstallPath != "/old" {
		t.Fatalf("unexpected legacy row: %+v", got)
	}

	// New writes work on the migrated table.
	if err := db.Model(&models.Server{}).Where("id = ?", 1).
		Update("launch_args", `{"foo":1}`).Error; err != nil {
		t.Fatalf("write new column on legacy db: %v", err)
	}
}

// TestInitializeLegacyModsFK reproduces the real startup crash: an existing
// hand-written `mods` table carrying a raw FOREIGN KEY clause makes the
// glebarez SQLite migrator choke ("table mods__temp has no column named
// FOREIGN") when AutoMigrate rebuilds it. Initialize must recover by dropping
// the empty legacy table and recreating it (AC4).
func TestInitializeLegacyModsFK(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy_mods.db")

	raw, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	legacyMods := `CREATE TABLE mods (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server_id INTEGER NOT NULL,
		workshop_id TEXT NOT NULL,
		name TEXT NOT NULL,
		enabled BOOLEAN DEFAULT 1,
		install_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
	);`
	if err := raw.Exec(legacyMods).Error; err != nil {
		t.Fatalf("seed legacy mods: %v", err)
	}
	if sqlDB, err := raw.DB(); err == nil {
		sqlDB.Close()
	}

	db, err := Initialize(dbPath)
	if err != nil {
		t.Fatalf("Initialize legacy-mods db: %v", err)
	}
	defer closeDB(t, db)

	// mods table is usable after recreation.
	if err := db.Create(&models.Mod{ServerID: 1, WorkshopID: "w", Name: "m"}).Error; err != nil {
		t.Fatalf("create mod after migrate: %v", err)
	}
}
