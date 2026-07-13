// Package oodle is a pure-Go, decompress-only port of the Oodle Kraken codec,
// sufficient to decode Palworld PlM save containers. It is a port of the
// decompression path of the open-source "ooz" reimplementation
// (temp/palsav/palooz/ooz/dep/ooz/kraken.cpp). Compression is not implemented.
//
// No cgo, no external native library: the whole codec is pure Go so the host
// application remains a single self-contained binary.
package oodle

import "errors"

// ErrUnsupported is returned when a block uses a codec/mode not yet ported
// (e.g. Mermaid/Selkie/Leviathan/LZNA/BitKnit). Palworld saves use Kraken.
var ErrUnsupported = errors.New("oodle: unsupported block type")

// Decompress decodes src (an Oodle stream) into exactly dstLen bytes.
func Decompress(src []byte, dstLen int) ([]byte, error) {
	dst := make([]byte, dstLen)
	n, err := krakenDecompress(src, dst)
	if err != nil {
		return nil, err
	}
	if n != dstLen {
		return nil, errors.New("oodle: decompressed size mismatch")
	}
	return dst, nil
}
