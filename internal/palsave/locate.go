package palsave

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoSave indicates no Palworld save (a world directory with a Level.sav, or
// a specific player's .sav) was found under the given path. Callers map this to
// a 404 "save not found" response.
var ErrNoSave = errors.New("palsave: no save found")

// LocateWorld finds the active world save directory under a server install
// path and returns the Level.sav path and the Players directory.
//
// Layout: <installPath>/Pal/Saved/SaveGames/0/<worldid>/{Level.sav, Players/}
// The <worldid> segment is normally a single directory; when several exist the
// first one containing a Level.sav is chosen. ErrNoSave is returned when the
// SaveGames/0 directory is missing or holds no world with a Level.sav.
func LocateWorld(installPath string) (levelPath, playersDir string, err error) {
	base := filepath.Join(installPath, "Pal", "Saved", "SaveGames", "0")
	entries, rerr := os.ReadDir(base)
	if rerr != nil {
		return "", "", ErrNoSave
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		world := filepath.Join(base, e.Name())
		lp := filepath.Join(world, "Level.sav")
		if fi, statErr := os.Stat(lp); statErr == nil && !fi.IsDir() {
			return lp, filepath.Join(world, "Players"), nil
		}
	}
	return "", "", ErrNoSave
}

// PlayerSaveFile derives a player's save file name from its PlayerUId.
// Palworld names player saves by the UID GUID rendered as 32 uppercase hex
// digits with the dashes removed, e.g.
//
//	"aabbccdd-0000-0000-0000-000000000000" -> "AABBCCDD000000000000000000000000.sav"
//
// (PlayerUId strings produced by the GVAS reader are lowercase and dashed.)
func PlayerSaveFile(uid string) string {
	hex := strings.ToUpper(strings.ReplaceAll(uid, "-", ""))
	return hex + ".sav"
}

// ResolvePlayerSave returns the path to a player's .sav under playersDir.
// It first tries the exact PlayerSaveFile name, then falls back to a
// case-insensitive directory scan (guarding against casing/format drift on
// case-sensitive filesystems). ErrNoSave is returned when no match exists.
func ResolvePlayerSave(playersDir, uid string) (string, error) {
	want := PlayerSaveFile(uid)
	exact := filepath.Join(playersDir, want)
	if fi, err := os.Stat(exact); err == nil && !fi.IsDir() {
		return exact, nil
	}
	entries, err := os.ReadDir(playersDir)
	if err != nil {
		return "", ErrNoSave
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), want) {
			return filepath.Join(playersDir, e.Name()), nil
		}
	}
	return "", ErrNoSave
}
