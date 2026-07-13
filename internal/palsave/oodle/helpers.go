package oodle

import "math/bits"

// Byte-level read/write helpers used throughout the Kraken port.
//
// The C reference reads unaligned little-endian words directly from the source
// buffer and deliberately over-reads a few bytes past logical stream ends (the
// extra bits are shifted out and never consumed). These helpers zero-pad any
// read that runs past the end of the provided slice, which reproduces that
// behaviour without out-of-bounds panics.

// rd16 reads a little-endian uint16 at b[p:], zero-padding past the end.
func rd16(b []byte, p int) uint32 {
	var v uint32
	if p >= 0 && p < len(b) {
		v |= uint32(b[p])
	}
	if p+1 >= 0 && p+1 < len(b) {
		v |= uint32(b[p+1]) << 8
	}
	return v
}

// rd32 reads a little-endian uint32 at b[p:], zero-padding past the end.
func rd32(b []byte, p int) uint32 {
	var v uint32
	for i := 0; i < 4; i++ {
		q := p + i
		if q >= 0 && q < len(b) {
			v |= uint32(b[q]) << uint(8*i)
		}
	}
	return v
}

// rd64 reads a little-endian uint64 at b[p:], zero-padding past the end.
func rd64(b []byte, p int) uint64 {
	var v uint64
	for i := 0; i < 8; i++ {
		q := p + i
		if q >= 0 && q < len(b) {
			v |= uint64(b[q]) << uint(8*i)
		}
	}
	return v
}

// bswap32 reads a big-endian uint32 at b[p:] (i.e. _byteswap_ulong(*(uint32*)p)).
func bswap32(b []byte, p int) uint32 {
	return bits.ReverseBytes32(rd32(b, p))
}

// st32 writes a little-endian uint32 to b[p:], ignoring out-of-range bytes.
func st32(b []byte, p int, v uint32) {
	for i := 0; i < 4; i++ {
		q := p + i
		if q >= 0 && q < len(b) {
			b[q] = byte(v >> uint(8*i))
		}
	}
}

// st64 writes a little-endian uint64 to b[p:], ignoring out-of-range bytes.
func st64(b []byte, p int, v uint64) {
	for i := 0; i < 8; i++ {
		q := p + i
		if q >= 0 && q < len(b) {
			b[q] = byte(v >> uint(8*i))
		}
	}
}

// bsr returns the index of the most significant set bit (C _BitScanReverse).
// Undefined for x==0 in C; callers guard against it.
func bsr(x uint32) int {
	return 31 - bits.LeadingZeros32(x)
}

// bsf returns the index of the least significant set bit (C _BitScanForward).
func bsf(x uint32) int {
	return bits.TrailingZeros32(x)
}
