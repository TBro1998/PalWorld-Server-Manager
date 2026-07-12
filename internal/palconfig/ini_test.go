package palconfig

import "testing"

func TestSplitTopLevel_RespectsQuotesAndParens(t *testing.T) {
	inner := `ServerName="a,b",CrossplayPlatforms=(Steam,Xbox,PS5,Mac),DeathPenalty=All`
	parts := splitTopLevel(inner)
	if len(parts) != 3 {
		t.Fatalf("expected 3 top-level parts, got %d: %#v", len(parts), parts)
	}
}

func TestParseOptionSettings_UnquotesStringsOnly(t *testing.T) {
	inner := `ServerName="My Server",DayTimeSpeedRate=1.000000,DeathPenalty=All,CrossplayPlatforms=(Steam,Xbox)`
	m := parseOptionSettings(inner)
	if m["ServerName"] != "My Server" {
		t.Errorf("ServerName not unquoted: %q", m["ServerName"])
	}
	if m["DayTimeSpeedRate"] != "1.000000" {
		t.Errorf("float mangled: %q", m["DayTimeSpeedRate"])
	}
	if m["DeathPenalty"] != "All" {
		t.Errorf("enum mangled: %q", m["DeathPenalty"])
	}
	if m["CrossplayPlatforms"] != "(Steam,Xbox)" {
		t.Errorf("raw tuple mangled: %q", m["CrossplayPlatforms"])
	}
}

func TestRoundTrip_PreservesComplexValues(t *testing.T) {
	inner := `ServerName="Has,Comma",CrossplayPlatforms=(Steam,Xbox,PS5,Mac),DenyTechnologyList=("PALBOX","RepairBench"),ExpRate=2.000000`
	m := parseOptionSettings(inner)
	out := serializeInner(m)
	m2 := parseOptionSettings(out)

	for _, k := range []string{"ServerName", "CrossplayPlatforms", "DenyTechnologyList", "ExpRate"} {
		if m[k] != m2[k] {
			t.Errorf("round-trip mismatch for %s: %q -> %q", k, m[k], m2[k])
		}
	}
}

func TestExtractOptionSettings(t *testing.T) {
	content := "[/Script/Pal.PalGameWorldSettings]\n" +
		`OptionSettings=(ServerName="X",ExpRate=1.000000)` + "\n"
	inner, ok := extractOptionSettings(content)
	if !ok {
		t.Fatal("expected to find OptionSettings")
	}
	m := parseOptionSettings(inner)
	if m["ServerName"] != "X" {
		t.Errorf("got %q", m["ServerName"])
	}
}

func TestReplaceOrAppendOption_Replaces(t *testing.T) {
	content := "[/Script/Pal.PalGameWorldSettings]\nOptionSettings=(ExpRate=1.000000)\n"
	out := replaceOrAppendOption(content, "OptionSettings=(ExpRate=5.000000)")
	inner, _ := extractOptionSettings(out)
	if parseOptionSettings(inner)["ExpRate"] != "5.000000" {
		t.Errorf("replace failed: %q", out)
	}
}

func TestBalancedParens(t *testing.T) {
	if balancedParens(`(a,(b,c))`) != 0 {
		t.Error("expected balanced")
	}
	if balancedParens(`(a,"("`+`)`) != 0 { // quote hides the inner '('
		t.Error("expected balanced with quoted paren")
	}
	if balancedParens(`(a,(b)`) == 0 {
		t.Error("expected unbalanced")
	}
}
