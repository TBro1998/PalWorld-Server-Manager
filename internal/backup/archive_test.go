package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFile creates a file with content, making parent dirs.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCreateZipAndExtractRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	saveDir := filepath.Join(tmp, "src", "save")
	configDir := filepath.Join(tmp, "src", "config")
	writeFile(t, filepath.Join(saveDir, "Level.sav"), "level-data")
	writeFile(t, filepath.Join(saveDir, "Players", "abc.sav"), "player-data")
	writeFile(t, filepath.Join(configDir, "PalWorldSettings.ini"), "ini-data")

	dest := filepath.Join(tmp, "backups", "1.zip")
	m := Manifest{ServerID: 1, Scope: "all", Source: "manual", CreatedAt: time.Unix(1000, 0)}
	if err := CreateZip(dest, Source{SaveDir: saveDir, ConfigDir: configDir}, m); err != nil {
		t.Fatalf("CreateZip: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("zip not created: %v", err)
	}

	// Manifest is readable and records both prefixes.
	got, err := ReadManifest(dest)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if got.ServerID != 1 || got.Scope != "all" {
		t.Fatalf("manifest mismatch: %+v", got)
	}
	if got.SavePrefix != savePrefix || got.ConfigPrefix != configPrefix {
		t.Fatalf("manifest prefixes: %+v", got)
	}

	// Restore into fresh (non-existent) destinations.
	dstSave := filepath.Join(tmp, "dst", "save")
	dstConfig := filepath.Join(tmp, "dst", "config")
	targets := []RestoreTarget{
		{Prefix: savePrefix, DestDir: dstSave},
		{Prefix: configPrefix, DestDir: dstConfig},
	}
	if err := ExtractInto(dest, targets); err != nil {
		t.Fatalf("ExtractInto: %v", err)
	}
	if c := readFile(t, filepath.Join(dstSave, "Level.sav")); c != "level-data" {
		t.Fatalf("restored Level.sav = %q", c)
	}
	if c := readFile(t, filepath.Join(dstSave, "Players", "abc.sav")); c != "player-data" {
		t.Fatalf("restored player save = %q", c)
	}
	if c := readFile(t, filepath.Join(dstConfig, "PalWorldSettings.ini")); c != "ini-data" {
		t.Fatalf("restored ini = %q", c)
	}
}

func TestExtractReplacesExistingDir(t *testing.T) {
	tmp := t.TempDir()
	saveDir := filepath.Join(tmp, "src", "save")
	writeFile(t, filepath.Join(saveDir, "Level.sav"), "new")

	dest := filepath.Join(tmp, "b.zip")
	if err := CreateZip(dest, Source{SaveDir: saveDir}, Manifest{ServerID: 1, Scope: "save"}); err != nil {
		t.Fatalf("CreateZip: %v", err)
	}

	// Pre-existing destination with a stale file that must be gone after restore.
	dst := filepath.Join(tmp, "dst", "save")
	writeFile(t, filepath.Join(dst, "stale.sav"), "old")
	writeFile(t, filepath.Join(dst, "Level.sav"), "old")

	if err := ExtractInto(dest, []RestoreTarget{{Prefix: savePrefix, DestDir: dst}}); err != nil {
		t.Fatalf("ExtractInto: %v", err)
	}
	if c := readFile(t, filepath.Join(dst, "Level.sav")); c != "new" {
		t.Fatalf("Level.sav = %q, want new", c)
	}
	if _, err := os.Stat(filepath.Join(dst, "stale.sav")); !os.IsNotExist(err) {
		t.Fatalf("stale file survived restore")
	}
}

func TestCreateZipExcludesGameBackupDir(t *testing.T) {
	tmp := t.TempDir()
	// saveDir mirrors the SaveGames/0 root: worlds sit one level below it, and
	// the game keeps its own rolling backups at <worldid>/backup.
	saveDir := filepath.Join(tmp, "src", "save")
	world := filepath.Join(saveDir, "582BEF41428843ECE7CACDA22D2A021C")
	writeFile(t, filepath.Join(world, "Level.sav"), "level-data")
	writeFile(t, filepath.Join(world, "Players", "abc.sav"), "player-data")
	writeFile(t, filepath.Join(world, "backup", "00000000", "Level.sav"), "game-backup")

	dest := filepath.Join(tmp, "b.zip")
	if err := CreateZip(dest, Source{SaveDir: saveDir}, Manifest{ServerID: 1, Scope: "save"}); err != nil {
		t.Fatalf("CreateZip: %v", err)
	}

	dst := filepath.Join(tmp, "dst", "save")
	if err := ExtractInto(dest, []RestoreTarget{{Prefix: savePrefix, DestDir: dst}}); err != nil {
		t.Fatalf("ExtractInto: %v", err)
	}
	// Real save content survives.
	if c := readFile(t, filepath.Join(dst, "582BEF41428843ECE7CACDA22D2A021C", "Level.sav")); c != "level-data" {
		t.Fatalf("Level.sav = %q, want level-data", c)
	}
	// The game's own backup subtree must not be in the archive.
	if _, err := os.Stat(filepath.Join(dst, "582BEF41428843ECE7CACDA22D2A021C", "backup")); !os.IsNotExist(err) {
		t.Fatalf("game backup dir was archived; want excluded")
	}
}

func TestCreateZipMissingSourceDirIsEmpty(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "b.zip")
	// SaveDir points at a non-existent path — should produce a valid (manifest-only) zip.
	err := CreateZip(dest, Source{SaveDir: filepath.Join(tmp, "nope")}, Manifest{ServerID: 1, Scope: "save"})
	if err != nil {
		t.Fatalf("CreateZip with missing dir: %v", err)
	}
	if _, err := ReadManifest(dest); err != nil {
		t.Fatalf("manifest unreadable: %v", err)
	}
}
