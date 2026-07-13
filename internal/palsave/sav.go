// Package palsave implements a pure-Go, read-only parser for Palworld save
// (.sav) files. It decompresses the container (zlib PlZ/CNK or Oodle Kraken
// PlM), parses the embedded GVAS blob, and extracts semantic data (players,
// pals, guilds, inventories).
//
// Reference implementation: temp/palsav (a fork of palworld-save-tools).
// This package is read-only: it never writes .sav files.
package palsave

import (
	"encoding/binary"
	"fmt"
)

// SaveType is the compression/container type identified by the SAV magic byte.
type SaveType byte

const (
	SaveTypeCNK SaveType = 0x30 // "CNK" — zlib, 24-byte double header
	SaveTypePLM SaveType = 0x31 // "PlM" — Oodle Kraken
	SaveTypePLZ SaveType = 0x32 // "PlZ" — double zlib
)

// savHeader is the parsed 12- (or 24-) byte SAV container header.
type savHeader struct {
	uncompressedLen uint32
	compressedLen   uint32
	magic           [3]byte
	saveType        SaveType
	dataOffset      int
}

var (
	magicPLZ = [3]byte{'P', 'l', 'Z'}
	magicPLM = [3]byte{'P', 'l', 'M'}
	magicCNK = [3]byte{'C', 'N', 'K'}
)

// parseHeader parses the SAV container header. Mirrors
// temp/palsav/palsav/compressor/__init__.py::_parse_sav_header.
func parseHeader(data []byte) (savHeader, error) {
	var h savHeader
	if len(data) < 24 {
		return h, fmt.Errorf("palsave: file too small to parse header (%d bytes)", len(data))
	}
	h.uncompressedLen = binary.LittleEndian.Uint32(data[0:4])
	h.compressedLen = binary.LittleEndian.Uint32(data[4:8])
	copy(h.magic[:], data[8:11])
	h.saveType = SaveType(data[11])
	h.dataOffset = 12

	// CNK stores placeholders in the first 12 bytes; the real header is at 12..24.
	if h.magic == magicCNK {
		h.uncompressedLen = binary.LittleEndian.Uint32(data[12:16])
		h.compressedLen = binary.LittleEndian.Uint32(data[16:20])
		copy(h.magic[:], data[20:23])
		h.saveType = SaveType(data[23])
		h.dataOffset = 24
	}

	switch h.magic {
	case magicPLZ, magicPLM, magicCNK:
		return h, nil
	default:
		return h, fmt.Errorf("palsave: unknown magic bytes %q", h.magic[:])
	}
}

// DecompressToGVAS parses the SAV header and returns the raw GVAS bytes.
// Supports PlZ/CNK (zlib) and PlM (Oodle Kraken).
func DecompressToGVAS(data []byte) ([]byte, SaveType, error) {
	h, err := parseHeader(data)
	if err != nil {
		return nil, 0, err
	}
	switch h.magic {
	case magicPLZ, magicCNK:
		out, err := decompressZlib(data, h)
		return out, h.saveType, err
	case magicPLM:
		out, err := decompressOodle(data, h)
		return out, h.saveType, err
	default:
		return nil, 0, fmt.Errorf("palsave: unsupported magic %q", h.magic[:])
	}
}
