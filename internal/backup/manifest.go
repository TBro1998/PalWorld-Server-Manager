// Package backup implements server save/config archiving: creating zip
// snapshots, applying retention policies, restoring, reconciling on-disk zips
// with DB records, and scheduling automatic backups. It performs only
// filesystem and policy computation — DB and HTTP concerns stay in the caller
// (internal/api), mirroring the palsave/palconfig split.
package backup

import (
	"archive/zip"
	"encoding/json"
	"io"
	"time"
)

// manifestName is the fixed entry written into every backup zip so an archive
// stays self-describing outside this tool.
const manifestName = "manifest.json"

// Manifest is the metadata embedded in a backup zip. It duplicates the DB row's
// key facts so a backup can be identified even if the database is lost.
type Manifest struct {
	ServerID  int64     `json:"server_id"`
	Scope     string    `json:"scope"`  // save | config | all
	Source    string    `json:"source"` // manual | auto | pre-restore
	Hot       bool      `json:"hot"`
	CreatedAt time.Time `json:"created_at"`
	// SavePrefix / ConfigPrefix are the top-level directories inside the zip that
	// hold the save tree and config dir respectively. Fixed constants, recorded
	// so a restore knows where to read from without guessing.
	SavePrefix   string `json:"save_prefix,omitempty"`
	ConfigPrefix string `json:"config_prefix,omitempty"`
}

// writeManifest serializes m into the given zip writer under manifestName.
func writeManifest(zw *zip.Writer, m Manifest) error {
	w, err := zw.Create(manifestName)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// ReadManifest reads and parses the manifest from a backup zip at path.
func ReadManifest(path string) (Manifest, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Manifest{}, err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != manifestName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return Manifest{}, err
		}
		defer rc.Close()
		var m Manifest
		if err := json.NewDecoder(rc).Decode(&m); err != nil {
			return Manifest{}, err
		}
		return m, nil
	}
	return Manifest{}, io.ErrUnexpectedEOF
}
