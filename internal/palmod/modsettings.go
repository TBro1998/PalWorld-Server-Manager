package palmod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	modSettingsSection = "[PalModSettings]"
	globalEnableKey    = "bGlobalEnableMod"
	activeModListKey   = "ActiveModList"
)

// EnabledMod is the minimal projection needed to render one ActiveModList line.
type EnabledMod struct {
	PackageName string
}

// ModSettingsPath returns <installPath>/Mods/PalModSettings.ini.
func ModSettingsPath(installPath string) string {
	return filepath.Join(ModsDir(installPath), "PalModSettings.ini")
}

// WriteModSettings writes/updates PalModSettings.ini so that, under the
// [PalModSettings] section, bGlobalEnableMod=true is set and there is exactly
// one ActiveModList=<PackageName> line per enabled mod. Other keys in the
// section and any other sections/comments in the file are preserved verbatim.
//
// The write is idempotent: repeated calls never accumulate duplicate managed
// lines (ActiveModList is a repeated/array-style key, so a folding INI library
// cannot be used — this is a line-oriented read/rewrite). A missing file is
// created (creating <installPath>/Mods first), matching the fact that Palworld
// only generates this file on first server launch.
func WriteModSettings(installPath string, enabled []EnabledMod) error {
	path := ModSettingsPath(installPath)

	var content string
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read PalModSettings.ini: %w", err)
	}

	updated := rebuildModSettings(content, enabled)

	if err := os.MkdirAll(ModsDir(installPath), 0o755); err != nil {
		return fmt.Errorf("create Mods dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write PalModSettings.ini: %w", err)
	}
	return nil
}

// rebuildModSettings produces the new file content: it strips the previously
// managed keys (bGlobalEnableMod / ActiveModList) inside [PalModSettings] and
// re-inserts the freshly rendered managed lines right after the section header,
// preserving all other content.
func rebuildModSettings(content string, enabled []EnabledMod) string {
	managed := managedLines(enabled)

	if strings.TrimSpace(content) == "" {
		return modSettingsSection + "\n" + strings.Join(managed, "\n") + "\n"
	}

	lines := strings.Split(content, "\n")

	sectionStart := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == modSettingsSection {
			sectionStart = i
			break
		}
	}

	// Section missing: append it (with managed lines) at the end of the file.
	if sectionStart < 0 {
		result := append([]string{}, lines...)
		result = append(result, modSettingsSection)
		result = append(result, managed...)
		return ensureTrailingNewline(strings.Join(result, "\n"))
	}

	// Find the end of the section: the next section header or EOF.
	sectionEnd := len(lines)
	for i := sectionStart + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			sectionEnd = i
			break
		}
	}

	// Keep the section's other (non-managed) lines.
	var kept []string
	for i := sectionStart + 1; i < sectionEnd; i++ {
		if isManagedKey(lines[i]) {
			continue
		}
		kept = append(kept, lines[i])
	}

	result := make([]string, 0, len(lines)+len(managed))
	result = append(result, lines[:sectionStart+1]...) // up to and incl. header
	result = append(result, managed...)
	result = append(result, kept...)
	result = append(result, lines[sectionEnd:]...)

	return ensureTrailingNewline(strings.Join(result, "\n"))
}

// managedLines renders the managed section body: the global enable flag plus
// one ActiveModList line per enabled mod (empty package names are skipped).
func managedLines(enabled []EnabledMod) []string {
	lines := []string{globalEnableKey + "=true"}
	for _, m := range enabled {
		if strings.TrimSpace(m.PackageName) == "" {
			continue
		}
		lines = append(lines, activeModListKey+"="+m.PackageName)
	}
	return lines
}

// isManagedKey reports whether a line assigns one of the keys this package owns.
func isManagedKey(line string) bool {
	t := strings.TrimSpace(line)
	eq := strings.Index(t, "=")
	if eq < 0 {
		return false
	}
	key := strings.TrimSpace(t[:eq])
	return key == globalEnableKey || key == activeModListKey
}

func ensureTrailingNewline(s string) string {
	if !strings.HasSuffix(s, "\n") {
		return s + "\n"
	}
	return s
}
