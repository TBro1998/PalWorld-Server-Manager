package palsave

import (
	"fmt"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/palsave/oodle"
)

// decompressOodle decompresses a PlM (Oodle Kraken) container.
// Mirrors temp/palsav/palsav/compressor/oozlib.py::decompress.
func decompressOodle(data []byte, h savHeader) ([]byte, error) {
	end := h.dataOffset + int(h.compressedLen)
	if end > len(data) {
		return nil, fmt.Errorf("palsave: compressed length %d exceeds file size", h.compressedLen)
	}
	comp := data[h.dataOffset:end]
	out, err := oodle.Decompress(comp, int(h.uncompressedLen))
	if err != nil {
		return nil, fmt.Errorf("palsave: oodle decompress: %w", err)
	}
	if len(out) != int(h.uncompressedLen) {
		return nil, fmt.Errorf("palsave: oodle output length %d != %d", len(out), h.uncompressedLen)
	}
	return out, nil
}
