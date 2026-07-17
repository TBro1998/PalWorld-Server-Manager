package palmod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readIni(t *testing.T, installPath string) string {
	t.Helper()
	data, err := os.ReadFile(ModSettingsPath(installPath))
	if err != nil {
		t.Fatalf("read ini: %v", err)
	}
	return string(data)
}

func countLines(content, prefix string) int {
	n := 0
	for _, ln := range strings.Split(content, "\n") {
		if strings.TrimSpace(ln) == prefix {
			n++
		}
	}
	return n
}

func TestWriteModSettingsCreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteModSettings(dir, []EnabledMod{{PackageName: "ModA"}, {PackageName: "ModB"}}); err != nil {
		t.Fatalf("write: %v", err)
	}
	content := readIni(t, dir)

	if !strings.Contains(content, modSettingsSection) {
		t.Errorf("missing section header:\n%s", content)
	}
	if countLines(content, "bGlobalEnableMod=true") != 1 {
		t.Errorf("expected exactly one bGlobalEnableMod=true:\n%s", content)
	}
	if countLines(content, "ActiveModList=ModA") != 1 || countLines(content, "ActiveModList=ModB") != 1 {
		t.Errorf("expected one ActiveModList line per mod:\n%s", content)
	}
}

func TestWriteModSettingsIdempotent(t *testing.T) {
	dir := t.TempDir()
	mods := []EnabledMod{{PackageName: "ModA"}, {PackageName: "ModB"}}
	for i := 0; i < 3; i++ {
		if err := WriteModSettings(dir, mods); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	content := readIni(t, dir)

	if got := countLines(content, "bGlobalEnableMod=true"); got != 1 {
		t.Errorf("bGlobalEnableMod duplicated: %d\n%s", got, content)
	}
	if got := countLines(content, "ActiveModList=ModA"); got != 1 {
		t.Errorf("ActiveModList=ModA duplicated: %d\n%s", got, content)
	}
	if got := countLines(content, "ActiveModList=ModB"); got != 1 {
		t.Errorf("ActiveModList=ModB duplicated: %d\n%s", got, content)
	}
	if got := countLines(content, modSettingsSection); got != 1 {
		t.Errorf("section duplicated: %d\n%s", got, content)
	}
}

func TestWriteModSettingsDisableSubset(t *testing.T) {
	dir := t.TempDir()
	if err := WriteModSettings(dir, []EnabledMod{{PackageName: "ModA"}, {PackageName: "ModB"}}); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Now only ModA is enabled.
	if err := WriteModSettings(dir, []EnabledMod{{PackageName: "ModA"}}); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	content := readIni(t, dir)

	if !strings.Contains(content, "ActiveModList=ModA") {
		t.Errorf("ModA should remain:\n%s", content)
	}
	if strings.Contains(content, "ActiveModList=ModB") {
		t.Errorf("ModB should have been removed:\n%s", content)
	}
}

func TestWriteModSettingsEmptyEnabled(t *testing.T) {
	dir := t.TempDir()
	if err := WriteModSettings(dir, nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	content := readIni(t, dir)
	if !strings.Contains(content, "bGlobalEnableMod=true") {
		t.Errorf("global enable should still be written:\n%s", content)
	}
	if strings.Contains(content, "ActiveModList=") {
		t.Errorf("no ActiveModList lines expected:\n%s", content)
	}
}

func TestWriteModSettingsPreservesOtherContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(ModsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := "; a comment\n" + modSettingsSection + "\nSomeOtherKey=42\nActiveModList=Stale\n\n[OtherSection]\nFoo=bar\n"
	if err := os.WriteFile(ModSettingsPath(dir), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteModSettings(dir, []EnabledMod{{PackageName: "ModA"}}); err != nil {
		t.Fatalf("write: %v", err)
	}
	content := readIni(t, dir)

	if !strings.Contains(content, "; a comment") {
		t.Errorf("comment lost:\n%s", content)
	}
	if !strings.Contains(content, "SomeOtherKey=42") {
		t.Errorf("other key in section lost:\n%s", content)
	}
	if !strings.Contains(content, "[OtherSection]") || !strings.Contains(content, "Foo=bar") {
		t.Errorf("other section lost:\n%s", content)
	}
	if strings.Contains(content, "ActiveModList=Stale") {
		t.Errorf("stale ActiveModList should be removed:\n%s", content)
	}
	if !strings.Contains(content, "ActiveModList=ModA") {
		t.Errorf("new ActiveModList missing:\n%s", content)
	}
}

func TestDeployAndRemove(t *testing.T) {
	install := t.TempDir()
	src := t.TempDir()
	// Build a small source tree.
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "Info.json"), []byte(`{"PackageName":"X"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "data.pak"), []byte("pak"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst, err := Deploy(install, "12345", src)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if dst != WorkshopDir(install, "12345") {
		t.Errorf("unexpected dst: %s", dst)
	}
	if _, err := os.Stat(filepath.Join(dst, "Info.json")); err != nil {
		t.Errorf("Info.json not deployed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "data.pak")); err != nil {
		t.Errorf("nested file not deployed: %v", err)
	}

	// Redeploy from a source missing the nested file: destination must mirror it.
	src2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(src2, "Info.json"), []byte(`{"PackageName":"X"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Deploy(install, "12345", src2); err != nil {
		t.Fatalf("redeploy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "data.pak")); !os.IsNotExist(err) {
		t.Errorf("stale nested file should be gone after redeploy: %v", err)
	}

	if err := Remove(install, "12345"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("workshop dir should be removed: %v", err)
	}
	// Remove is a no-op on a missing directory.
	if err := Remove(install, "12345"); err != nil {
		t.Errorf("remove of missing dir should be nil: %v", err)
	}
}

func TestParseInfoTolerant(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Info.json"),
		[]byte(`{"ModName":"My Mod","PackageName":"MyMod","Version":"1.2.3","Tags":["Gameplay","QoL"],"InstallRule":[{"IsServer":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := ParseInfo(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.PackageName != "MyMod" || info.ModName != "My Mod" || info.Version != "1.2.3" || !info.IsServer {
		t.Errorf("unexpected info: %+v", info)
	}
	if len(info.Tags) != 2 || info.Tags[0] != "Gameplay" || info.Tags[1] != "QoL" {
		t.Errorf("unexpected tags: %#v", info.Tags)
	}
}

func TestParseInfoNumericVersionAndMissingFields(t *testing.T) {
	dir := t.TempDir()
	// Numeric version, no InstallRule at all.
	if err := os.WriteFile(filepath.Join(dir, "Info.json"),
		[]byte(`{"PackageName":"Mod2","Version":7}`), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := ParseInfo(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.Version != "7" {
		t.Errorf("numeric version not decoded: %q", info.Version)
	}
	if info.IsServer {
		t.Errorf("IsServer should default to false when no rules present")
	}
	// Absent ModName/Tags must be zero-valued, not an error.
	if info.ModName != "" {
		t.Errorf("ModName should default to empty: %q", info.ModName)
	}
	if info.Tags != nil {
		t.Errorf("Tags should default to nil when absent: %#v", info.Tags)
	}
}

func TestParseInfoMissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := ParseInfo(dir); err == nil {
		t.Errorf("expected error for missing Info.json")
	}
}
