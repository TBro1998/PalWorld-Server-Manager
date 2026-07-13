package palsave

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// zlibInflate performs a single zlib decompression pass.
func zlibInflate(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// decompressZlib handles PlZ (double zlib) and CNK (single zlib) containers.
// Mirrors temp/palsav/palsav/compressor/zlib.py::decompress.
func decompressZlib(data []byte, h savHeader) ([]byte, error) {
	out, err := zlibInflate(data[h.dataOffset:])
	if err != nil {
		return nil, fmt.Errorf("palsave: zlib inflate (pass 1): %w", err)
	}
	// PlZ (0x32) is compressed twice; the first inflate yields the inner
	// compressed stream whose length must equal compressedLen.
	if h.saveType == SaveTypePLZ {
		if uint32(len(out)) != h.compressedLen {
			return nil, fmt.Errorf("palsave: incorrect intermediate length %d != %d", len(out), h.compressedLen)
		}
		out, err = zlibInflate(out)
		if err != nil {
			return nil, fmt.Errorf("palsave: zlib inflate (pass 2): %w", err)
		}
	}
	if uint32(len(out)) != h.uncompressedLen {
		return nil, fmt.Errorf("palsave: incorrect uncompressed length %d != %d", len(out), h.uncompressedLen)
	}
	return out, nil
}
