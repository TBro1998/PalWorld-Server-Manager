package palsave

import (
	"os"
	"path/filepath"
	"testing"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Skipf("testdata/%s unavailable: %v", name, err)
	}
	return b
}

func TestParseHeader(t *testing.T) {
	cases := []struct {
		file             string
		wantUncompressed uint32
		wantSaveType     SaveType
	}{
		{"Player.sav", 0x000025be, SaveTypePLM},
		{"LevelMeta.sav", 0x0000084a, SaveTypePLM},
		{"Level.sav", 0x00081048, SaveTypePLM},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			data := readTestdata(t, c.file)
			h, err := parseHeader(data)
			if err != nil {
				t.Fatalf("parseHeader: %v", err)
			}
			if h.magic != magicPLM {
				t.Errorf("magic = %q, want PlM", h.magic[:])
			}
			if h.saveType != c.wantSaveType {
				t.Errorf("saveType = %#x, want %#x", h.saveType, c.wantSaveType)
			}
			if h.uncompressedLen != c.wantUncompressed {
				t.Errorf("uncompressedLen = %d, want %d", h.uncompressedLen, c.wantUncompressed)
			}
			if h.dataOffset != 12 {
				t.Errorf("dataOffset = %d, want 12", h.dataOffset)
			}
			// compressed length must fit inside the file
			if int(h.compressedLen)+h.dataOffset > len(data) {
				t.Errorf("compressedLen %d + offset %d exceeds file %d", h.compressedLen, h.dataOffset, len(data))
			}
		})
	}
}

// TestDecompressOodle asserts the Oodle (PlM) path decodes to a GVAS blob.
func TestDecompressOodle(t *testing.T) {
	data := readTestdata(t, "LevelMeta.sav")
	out, _, err := DecompressToGVAS(data)
	if err != nil {
		t.Fatalf("Oodle decode failed: %v", err)
	}
	if len(out) < 4 || string(out[:4]) != "GVAS" {
		t.Fatalf("decompressed output does not start with GVAS: % x", out[:min(8, len(out))])
	}
	t.Logf("decompressed %d bytes, GVAS OK", len(out))
}

// TestDecompressAll decompresses every testdata save and asserts the exact
// uncompressed length and the GVAS magic header.
func TestDecompressAll(t *testing.T) {
	cases := []struct {
		file    string
		wantLen int
	}{
		{"LevelMeta.sav", 2122},
		{"Player.sav", 9662},
		{"Level.sav", 528456},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			data := readTestdata(t, c.file)
			out, st, err := DecompressToGVAS(data)
			if err != nil {
				t.Fatalf("DecompressToGVAS: %v", err)
			}
			if st != SaveTypePLM {
				t.Errorf("saveType = %#x, want PlM", st)
			}
			if len(out) != c.wantLen {
				t.Fatalf("len(out) = %d, want %d", len(out), c.wantLen)
			}
			if string(out[:4]) != "GVAS" {
				t.Fatalf("output does not start with GVAS: % x", out[:min(8, len(out))])
			}
			t.Logf("%s: %d bytes, GVAS OK", c.file, len(out))
		})
	}
}
