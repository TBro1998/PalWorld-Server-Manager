package palconfig

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const optionSettingsPrefix = "OptionSettings="

const settingsSection = "[/Script/Pal.PalGameWorldSettings]"

// embeddedDefaultINI is the fallback seed used when the server ships no
// DefaultPalWorldSettings.ini. It is generated from the parameter registry so
// the registry remains the single source of truth (no duplicated INI file).
var embeddedDefaultINI = settingsSection + "\n" +
	optionSettingsPrefix + "(" + serializeInner(Defaults()) + ")\n"

// ConfigDir returns the platform-specific directory that holds
// PalWorldSettings.ini for a given server install path.
//
//	Windows: <installPath>/Pal/Saved/Config/WindowsServer
//	Linux:   <installPath>/Pal/Saved/Config/LinuxServer
func ConfigDir(installPath string) string {
	sub := "LinuxServer"
	if runtime.GOOS == "windows" {
		sub = "WindowsServer"
	}
	return filepath.Join(installPath, "Pal", "Saved", "Config", sub)
}

// SettingsPath returns the full path to PalWorldSettings.ini.
func SettingsPath(installPath string) string {
	return filepath.Join(ConfigDir(installPath), "PalWorldSettings.ini")
}

// defaultSourcePath returns the path to the server-shipped
// DefaultPalWorldSettings.ini, if any.
func defaultSourcePath(installPath string) string {
	return filepath.Join(installPath, "DefaultPalWorldSettings.ini")
}

// ensureFile makes sure PalWorldSettings.ini exists, seeding it from the
// server's DefaultPalWorldSettings.ini when available, otherwise from the
// embedded template. Returns the file's content.
func ensureFile(installPath string) (string, error) {
	path := SettingsPath(installPath)
	if data, err := os.ReadFile(path); err == nil {
		return string(data), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read settings: %w", err)
	}

	// Seed content: prefer the server's default, fall back to embedded template.
	seed := embeddedDefaultINI
	if data, err := os.ReadFile(defaultSourcePath(installPath)); err == nil {
		seed = string(data)
	}

	if err := os.MkdirAll(ConfigDir(installPath), 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		return "", fmt.Errorf("seed settings: %w", err)
	}
	return seed, nil
}

// LoadSettings reads (seeding if needed) PalWorldSettings.ini and returns the
// effective OptionSettings values, unioned with registry defaults so every
// known parameter is present.
func LoadSettings(installPath string) (map[string]string, error) {
	content, err := ensureFile(installPath)
	if err != nil {
		return nil, err
	}

	values := Defaults()
	if inner, ok := extractOptionSettings(content); ok {
		for k, v := range parseOptionSettings(inner) {
			values[k] = v
		}
	}
	// REST API is always enabled; enforce unconditionally so stale INI files
	// (seeded from game-shipped DefaultPalWorldSettings.ini with False) are
	// treated as True without requiring a manual config save.
	values["RESTAPIEnabled"] = "True"
	return values, nil
}

// LoadRaw returns the current OptionSettings=(...) line (seeding if needed).
func LoadRaw(installPath string) (string, error) {
	content, err := ensureFile(installPath)
	if err != nil {
		return "", err
	}
	if inner, ok := extractOptionSettings(content); ok {
		return optionSettingsPrefix + "(" + inner + ")", nil
	}
	return RawLine(Defaults()), nil
}

// RawLine renders values as a complete OptionSettings=(...) line.
func RawLine(values map[string]string) string {
	return optionSettingsPrefix + "(" + serializeInner(values) + ")"
}

// SaveSettings serializes values into the OptionSettings line and writes the
// file back, preserving all other content.
// RESTAPIEnabled is always forced to True before writing — the UI toggle has
// been removed and the API is unconditionally on.
func SaveSettings(installPath string, values map[string]string) error {
	content, err := ensureFile(installPath)
	if err != nil {
		return err
	}
	// Enforce REST API always-on without mutating the caller's map.
	if values["RESTAPIEnabled"] != "True" {
		clone := make(map[string]string, len(values))
		maps.Copy(clone, values)
		clone["RESTAPIEnabled"] = "True"
		values = clone
	}
	line := optionSettingsPrefix + "(" + serializeInner(values) + ")"
	return writeFileAtomic(SettingsPath(installPath), replaceOrAppendOption(content, line))
}

// SaveRaw validates and writes a full OptionSettings=(...) line verbatim.
func SaveRaw(installPath, rawLine string) error {
	line := strings.TrimSpace(rawLine)
	if !strings.HasPrefix(line, optionSettingsPrefix) {
		return fmt.Errorf("raw config must start with %q", optionSettingsPrefix)
	}
	val := strings.TrimSpace(strings.TrimPrefix(line, optionSettingsPrefix))
	if !strings.HasPrefix(val, "(") || balancedParens(val) != 0 {
		return fmt.Errorf("raw config parentheses are unbalanced")
	}
	content, err := ensureFile(installPath)
	if err != nil {
		return err
	}
	return writeFileAtomic(SettingsPath(installPath), replaceOrAppendOption(content, line))
}

// --- parsing / serialization ---

// extractOptionSettings finds the OptionSettings line and returns the content
// between its outermost parentheses.
func extractOptionSettings(content string) (string, bool) {
	for _, ln := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(ln)
		if !strings.HasPrefix(trimmed, optionSettingsPrefix) {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, optionSettingsPrefix))
		return innerParen(val), true
	}
	return "", false
}

// innerParen returns the content between the first '(' and its matching ')'.
func innerParen(s string) string {
	start := strings.Index(s, "(")
	if start < 0 {
		return ""
	}
	depth := 0
	inQuote := false
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '(':
			if !inQuote {
				depth++
			}
		case ')':
			if !inQuote {
				depth--
				if depth == 0 {
					return s[start+1 : i]
				}
			}
		}
	}
	return s[start+1:]
}

// parseOptionSettings splits the inner OptionSettings content into key/value
// pairs, respecting quotes and nested parentheses. String-typed values are
// unquoted; everything else is kept verbatim.
func parseOptionSettings(inner string) map[string]string {
	m := make(map[string]string)
	for _, tok := range splitTopLevel(inner) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		eq := strings.Index(tok, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(tok[:eq])
		val := strings.TrimSpace(tok[eq+1:])
		if def, ok := Lookup(key); ok && def.Type == TypeString {
			val = unquote(val)
		}
		m[key] = val
	}
	return m
}

// splitTopLevel splits s on commas that are not inside quotes or parentheses.
func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	inQuote := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '(':
			if !inQuote {
				depth++
			}
		case ')':
			if !inQuote && depth > 0 {
				depth--
			}
		case ',':
			if !inQuote && depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// serializeInner renders values into the comma-joined OptionSettings body,
// registry order first then any unknown keys (sorted, verbatim).
func serializeInner(values map[string]string) string {
	var b strings.Builder
	first := true
	write := func(key, val string) {
		if !first {
			b.WriteByte(',')
		}
		first = false
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(val)
	}

	seen := make(map[string]bool, len(values))
	for _, p := range Params {
		if v, ok := values[p.Key]; ok {
			write(p.Key, serializeValue(p.Type, v))
			seen[p.Key] = true
		}
	}
	var extra []string
	for k := range values {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		write(k, values[k]) // unknown params passed through verbatim
	}
	return b.String()
}

func serializeValue(t ParamType, val string) string {
	if t == TypeString {
		return `"` + val + `"`
	}
	return val
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// balancedParens returns the paren depth remainder (0 == balanced), ignoring
// parentheses inside double quotes.
func balancedParens(s string) int {
	depth := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '(':
			if !inQuote {
				depth++
			}
		case ')':
			if !inQuote {
				depth--
			}
		}
	}
	return depth
}

// replaceOrAppendOption replaces the existing OptionSettings line with line, or
// appends it (under the settings section header) if none exists.
func replaceOrAppendOption(content, line string) string {
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), optionSettingsPrefix) {
			lines[i] = line
			return strings.Join(lines, "\n")
		}
	}
	// No existing line: ensure the section header exists, then append.
	if !strings.Contains(content, settingsSection) {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += settingsSection + "\n"
		lines = strings.Split(content, "\n")
	}
	for i, ln := range lines {
		if strings.TrimSpace(ln) == settingsSection {
			rest := append([]string{lines[i], line}, lines[i+1:]...)
			return strings.Join(append(lines[:i], rest...), "\n")
		}
	}
	return content + line + "\n"
}

func writeFileAtomic(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".palworldsettings-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
