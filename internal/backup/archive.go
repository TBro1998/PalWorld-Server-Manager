package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Fixed top-level prefixes inside a backup zip. Restore reads from these.
const (
	savePrefix   = "save"   // holds the SaveGames/0 tree
	configPrefix = "config" // holds the config dir contents
)

// Source describes what to archive. Either directory may be empty, in which case
// it is skipped (e.g. scope=save leaves ConfigDir empty). Paths are absolute
// directories on disk.
type Source struct {
	SaveDir   string // e.g. <install>/Pal/Saved/SaveGames/0  (empty to skip)
	ConfigDir string // e.g. <install>/Pal/Saved/Config/WindowsServer (empty to skip)
}

// CreateZip writes a backup zip at dest containing the requested source
// directories plus a manifest. Parent directories of dest are created. The zip
// is written to a temp file and renamed into place so a crash never leaves a
// half-written archive at dest.
func CreateZip(dest string, src Source, m Manifest) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".backup-*.zip.tmp")
	if err != nil {
		return fmt.Errorf("create temp zip: %w", err)
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup if we bail before the rename.
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	zw := zip.NewWriter(tmp)

	if src.SaveDir != "" {
		if err := addDir(zw, src.SaveDir, savePrefix, skipGameBackup); err != nil {
			zw.Close()
			tmp.Close()
			return fmt.Errorf("archive save dir: %w", err)
		}
		m.SavePrefix = savePrefix
	}
	if src.ConfigDir != "" {
		if err := addDir(zw, src.ConfigDir, configPrefix, nil); err != nil {
			zw.Close()
			tmp.Close()
			return fmt.Errorf("archive config dir: %w", err)
		}
		m.ConfigPrefix = configPrefix
	}

	if err := writeManifest(zw, m); err != nil {
		zw.Close()
		tmp.Close()
		return fmt.Errorf("write manifest: %w", err)
	}

	if err := zw.Close(); err != nil {
		tmp.Close()
		return fmt.Errorf("finalize zip: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp zip: %w", err)
	}

	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("finalize backup: %w", err)
	}
	tmpPath = "" // ownership transferred; skip cleanup
	return nil
}

// skip reports whether the entry at rel (a forward-slash relative path from the
// walk root) should be excluded from the archive. For a directory it prunes the
// whole subtree. A nil skip includes everything.
type skipFunc func(rel string, isDir bool) bool

// skipGameBackup excludes Palworld's own rolling save backups, which live at
// SaveGames/0/<worldid>/backup. Those are the game engine's backups, not this
// tool's, so folding them into our zips would bloat every archive and nest
// backups inside backups. rel is relative to the SaveGames/0 root, so the game
// backup dir is any path whose second segment is "backup".
func skipGameBackup(rel string, isDir bool) bool {
	parts := strings.Split(rel, "/")
	return len(parts) >= 2 && parts[1] == "backup"
}

// addDir walks root and writes every regular file into the zip under
// prefix/<relpath>. Directories are represented implicitly by their files'
// paths (empty dirs are added explicitly so structure is preserved). Symlinks
// are skipped. Entries for which skip returns true are pruned (skip may be nil).
// A missing root is treated as an empty tree (no error) so scope=all on a server
// that has a save but no custom config still succeeds.
func addDir(zw *zip.Writer, root, prefix string, skip skipFunc) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", root)
	}

	return filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip symlinks entirely (Walk reports them as non-dir; avoid following).
		if fi.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if skip != nil && rel != "." && skip(relSlash, fi.IsDir()) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// zip paths always use forward slashes.
		name := prefix
		if rel != "." {
			name = prefix + "/" + relSlash
		}

		if fi.IsDir() {
			// Write an explicit directory entry (trailing slash) to preserve
			// empty directories.
			if rel == "." {
				return nil // the prefix root itself is implicit
			}
			_, err := zw.CreateHeader(&zip.FileHeader{
				Name:     name + "/",
				Method:   zip.Store,
				Modified: fi.ModTime(),
			})
			return err
		}

		hdr, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		hdr.Name = name
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}

// RestoreTarget maps a zip prefix to the on-disk directory it should replace.
type RestoreTarget struct {
	Prefix string // savePrefix or configPrefix
	DestDir string // absolute directory to replace with the archived contents
}

// ExtractInto restores the archived directories from the zip at path into the
// given targets. For each target, the archived subtree under Prefix is unpacked
// into a sibling temp directory and then atomically swapped in for DestDir
// (old DestDir moved aside and removed on success). If any target fails, the
// already-swapped targets are left in place (each swap is independent); callers
// take a pre-restore backup first so a partial restore is still recoverable.
//
// A target whose prefix is absent from the zip is skipped (not an error): e.g.
// restoring a scope=save backup only touches the save target.
func ExtractInto(path string, targets []RestoreTarget) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer zr.Close()

	for _, t := range targets {
		if !hasPrefix(zr.File, t.Prefix) {
			continue
		}
		if err := restoreOne(zr.File, t); err != nil {
			return err
		}
	}
	return nil
}

// hasPrefix reports whether any zip entry lives under prefix/.
func hasPrefix(files []*zip.File, prefix string) bool {
	p := prefix + "/"
	for _, f := range files {
		if f.Name == prefix || strings.HasPrefix(f.Name, p) {
			return true
		}
	}
	return false
}

// restoreOne unpacks the subtree under t.Prefix into a staging dir on the same
// volume as t.DestDir, then atomically swaps it in.
func restoreOne(files []*zip.File, t RestoreTarget) error {
	parent := filepath.Dir(t.DestDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("prepare restore parent: %w", err)
	}

	staging, err := os.MkdirTemp(parent, ".restore-*")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	// Clean up staging unless we successfully hand it to DestDir.
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()

	prefixSlash := t.Prefix + "/"
	for _, f := range files {
		if !strings.HasPrefix(f.Name, prefixSlash) {
			continue
		}
		rel := strings.TrimPrefix(f.Name, prefixSlash)
		if rel == "" {
			continue
		}
		if err := extractEntry(f, staging, rel); err != nil {
			return err
		}
	}

	// Atomic-ish swap: move current DestDir aside, move staging into place,
	// then remove the old one. On Windows os.Rename fails if the target exists,
	// so DestDir must be moved away first.
	backupOld := t.DestDir + ".old-" + tmpSuffix()
	destExists := false
	if _, err := os.Stat(t.DestDir); err == nil {
		destExists = true
		if err := os.Rename(t.DestDir, backupOld); err != nil {
			return fmt.Errorf("move current dir aside: %w", err)
		}
	}
	if err := os.Rename(staging, t.DestDir); err != nil {
		// Roll back: put the old dir back.
		if destExists {
			_ = os.Rename(backupOld, t.DestDir)
		}
		return fmt.Errorf("swap in restored dir: %w", err)
	}
	committed = true
	if destExists {
		_ = os.RemoveAll(backupOld)
	}
	return nil
}

// extractEntry writes a single zip entry into staging/rel, guarding against
// path traversal (zip-slip).
func extractEntry(f *zip.File, staging, rel string) error {
	target := filepath.Join(staging, filepath.FromSlash(rel))
	// Zip-slip guard: the cleaned target must stay within staging.
	if !strings.HasPrefix(target, filepath.Clean(staging)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal path in archive: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// tmpSuffix returns a filesystem-safe unique-ish suffix for the aside dir.
// Uses nanosecond time; collisions across a single restore are impossible since
// restores of the same dir are serialized by the caller (server must be stopped).
func tmpSuffix() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}
