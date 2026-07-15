package palmod

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ModsDir returns <installPath>/Mods, the root of the server-side mod layout.
func ModsDir(installPath string) string {
	return filepath.Join(installPath, "Mods")
}

// WorkshopDir returns <installPath>/Mods/Workshop/<workshopID>, the per-mod
// deploy target.
func WorkshopDir(installPath, workshopID string) string {
	return filepath.Join(ModsDir(installPath), "Workshop", workshopID)
}

// Deploy copies the downloaded workshop content from srcDir into
// <installPath>/Mods/Workshop/<workshopID>/, returning the destination path.
// The destination is cleared first so it always mirrors the source exactly
// (removing files dropped in a newer version of the mod).
func Deploy(installPath, workshopID, srcDir string) (string, error) {
	info, err := os.Stat(srcDir)
	if err != nil {
		return "", fmt.Errorf("stat source: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source is not a directory: %s", srcDir)
	}

	dst := WorkshopDir(installPath, workshopID)
	if err := os.RemoveAll(dst); err != nil {
		return "", fmt.Errorf("clear destination: %w", err)
	}
	if err := copyTree(srcDir, dst); err != nil {
		return "", fmt.Errorf("copy mod content: %w", err)
	}
	return dst, nil
}

// Remove deletes <installPath>/Mods/Workshop/<workshopID>/ (FR5). A missing
// directory is not an error.
func Remove(installPath, workshopID string) error {
	if err := os.RemoveAll(WorkshopDir(installPath, workshopID)); err != nil {
		return fmt.Errorf("remove mod directory: %w", err)
	}
	return nil
}

// copyTree recursively copies the directory tree rooted at src into dst,
// creating dst (and parents) as needed.
func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyTree(s, d); err != nil {
				return err
			}
			continue
		}
		// Skip non-regular entries (e.g. symlinks) rather than fail the deploy.
		info, err := e.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
