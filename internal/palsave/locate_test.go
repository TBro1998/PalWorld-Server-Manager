package palsave

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlayerSaveFile(t *testing.T) {
	got := PlayerSaveFile("aabbccdd-0000-0000-0000-000000000000")
	want := "AABBCCDD000000000000000000000000.sav"
	if got != want {
		t.Fatalf("PlayerSaveFile = %q, want %q", got, want)
	}
}

func TestLocateWorld(t *testing.T) {
	root := t.TempDir()
	world := filepath.Join(root, "Pal", "Saved", "SaveGames", "0", "ABCDEF")
	if err := os.MkdirAll(filepath.Join(world, "Players"), 0o755); err != nil {
		t.Fatal(err)
	}
	level := readTestdata(t, "Level.sav")
	if err := os.WriteFile(filepath.Join(world, "Level.sav"), level, 0o644); err != nil {
		t.Fatal(err)
	}

	lp, pd, err := LocateWorld(root)
	if err != nil {
		t.Fatalf("LocateWorld: %v", err)
	}
	if lp != filepath.Join(world, "Level.sav") {
		t.Errorf("levelPath = %q, want %q", lp, filepath.Join(world, "Level.sav"))
	}
	if pd != filepath.Join(world, "Players") {
		t.Errorf("playersDir = %q, want %q", pd, filepath.Join(world, "Players"))
	}
}

func TestLocateWorldNoSave(t *testing.T) {
	root := t.TempDir()
	if _, _, err := LocateWorld(root); err != ErrNoSave {
		t.Fatalf("err(missing) = %v, want ErrNoSave", err)
	}
	// SaveGames/0 exists but the world dir has no Level.sav -> still ErrNoSave.
	if err := os.MkdirAll(filepath.Join(root, "Pal", "Saved", "SaveGames", "0", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LocateWorld(root); err != ErrNoSave {
		t.Fatalf("err(empty world) = %v, want ErrNoSave", err)
	}
}

func TestResolvePlayerSaveExact(t *testing.T) {
	dir := t.TempDir()
	uid := "11223344-5566-7788-99aa-bbccddeeff00"
	exact := filepath.Join(dir, PlayerSaveFile(uid))
	if err := os.WriteFile(exact, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolvePlayerSave(dir, uid)
	if err != nil {
		t.Fatalf("ResolvePlayerSave: %v", err)
	}
	if got != exact {
		t.Errorf("got %q, want %q", got, exact)
	}
}

func TestResolvePlayerSaveScanFallback(t *testing.T) {
	dir := t.TempDir()
	uid := "11223344-5566-7788-99aa-bbccddeeff00"
	// Stored lowercased: on a case-sensitive FS the exact stat misses and the
	// scan fallback (EqualFold) resolves it; on a case-insensitive FS the exact
	// stat already hits. Either way it must resolve.
	name := strings.ToLower(PlayerSaveFile(uid))
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolvePlayerSave(dir, uid)
	if err != nil {
		t.Fatalf("ResolvePlayerSave: %v", err)
	}
	if !strings.EqualFold(filepath.Base(got), PlayerSaveFile(uid)) {
		t.Errorf("resolved to %q, want case-insensitive match of %q", filepath.Base(got), PlayerSaveFile(uid))
	}
}

func TestResolvePlayerSaveMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := ResolvePlayerSave(dir, "aabbccdd-0000-0000-0000-000000000000"); err != ErrNoSave {
		t.Fatalf("err = %v, want ErrNoSave", err)
	}
}
