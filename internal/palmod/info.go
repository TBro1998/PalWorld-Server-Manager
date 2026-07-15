// Package palmod holds the pure file/INI logic for deploying Palworld workshop
// mods into a dedicated-server install directory and maintaining the mod load
// configuration (PalModSettings.ini). It has no database or API dependency so it
// can be unit-tested in isolation with t.TempDir().
//
// Layout (Windows dedicated server, the only officially supported target):
//
//	<installPath>/Mods/Workshop/<workshopID>/   -- deployed mod content (+ Info.json)
//	<installPath>/Mods/PalModSettings.ini       -- load configuration
//
// All paths are assembled with path/filepath. Windows is prioritized, but the
// code is OS-agnostic (no build tags needed) so Linux compilation is preserved.
package palmod

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Info is the tolerant projection of a mod's Info.json that we care about.
// PackageName drives ActiveModList, Version drives update detection, and
// IsServer (derived from InstallRules) drives the FR9 compatibility warning.
type Info struct {
	PackageName string
	Version     string
	IsServer    bool
}

// rawInfo mirrors the parts of Info.json we read. The exact key names come from
// the official docs but no real sample was verified during implementation, so
// parsing is deliberately tolerant: case variants are accepted via multiple
// struct tags where practical, missing fields never panic, and Version accepts
// either a string or a number. See design.md risk #1.
type rawInfo struct {
	PackageName  string           `json:"PackageName"`
	Version      json.RawMessage  `json:"Version"`
	InstallRules []rawInstallRule `json:"InstallRule"`
	// InstallRules is an alternate spelling seen in some samples; harmless if absent.
	InstallRulesAlt []rawInstallRule `json:"InstallRules"`
}

type rawInstallRule struct {
	IsServer bool `json:"IsServer"`
}

// ParseInfo reads <dir>/Info.json and returns a tolerant Info projection.
// A missing file or unparseable JSON returns an error; missing individual
// fields are simply left zero-valued (IsServer defaults to false, which triggers
// the FR9 "may not be designed for servers" warning).
func ParseInfo(dir string) (*Info, error) {
	path := filepath.Join(dir, "Info.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Info.json: %w", err)
	}

	var raw rawInfo
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse Info.json: %w", err)
	}

	info := &Info{
		PackageName: raw.PackageName,
		Version:     decodeVersion(raw.Version),
	}

	rules := raw.InstallRules
	if len(rules) == 0 {
		rules = raw.InstallRulesAlt
	}
	for _, r := range rules {
		if r.IsServer {
			info.IsServer = true
			break
		}
	}
	return info, nil
}

// decodeVersion renders the Version field whether it arrived as a JSON string
// (e.g. "1.0.0") or a number (e.g. 3). Absent/invalid values yield "".
func decodeVersion(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return ""
}
