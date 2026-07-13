package oodle

import "math/bits"

// Ported from ooz kraken.cpp (BitReader + header parsers). Read-only, Kraken path.
//
// C intrinsics mapped to Go:
//   _BitScanReverse(x) -> 31 - bits.LeadingZeros32(x)   (index of highest set bit)
//   _rotl(x, n)        -> bits.RotateLeft32(x, n)
//   _byteswap_ushort   -> bits.ReverseBytes16

// bitReader mirrors struct BitReader. p is an index into the backing slice buf.
type bitReader struct {
	buf    []byte
	p      int // index of next byte to consume (forward) / one-past for backward
	pEnd   int // boundary index
	bits   uint32
	bitpos int
}

func (b *bitReader) refill() {
	// while bitpos > 0: bits |= (p<pEnd ? buf[p] : 0) << bitpos; bitpos -= 8; p++
	for b.bitpos > 0 {
		var v uint32
		if b.p < b.pEnd {
			v = uint32(b.buf[b.p])
		}
		b.bits |= v << uint(b.bitpos)
		b.bitpos -= 8
		b.p++
	}
}

func (b *bitReader) refillBackwards() {
	for b.bitpos > 0 {
		b.p--
		var v uint32
		if b.p >= b.pEnd {
			v = uint32(b.buf[b.p])
		}
		b.bits |= v << uint(b.bitpos)
		b.bitpos -= 8
	}
}

func (b *bitReader) readBit() int {
	b.refill()
	r := int(b.bits >> 31)
	b.bits <<= 1
	b.bitpos++
	return r
}

func (b *bitReader) readBitNoRefill() int {
	r := int(b.bits >> 31)
	b.bits <<= 1
	b.bitpos++
	return r
}

func (b *bitReader) readBitsNoRefill(n int) int {
	r := int(b.bits >> uint(32-n))
	b.bits <<= uint(n)
	b.bitpos += n
	return r
}

func (b *bitReader) readBitsNoRefillZero(n int) int {
	// (bits >> 1 >> (31 - n))
	r := int(b.bits >> 1 >> uint(31-n))
	b.bits <<= uint(n)
	b.bitpos += n
	return r
}

func (b *bitReader) readMoreThan24Bits(n int) uint32 {
	var rv uint32
	if n <= 24 {
		rv = uint32(b.readBitsNoRefillZero(n))
	} else {
		rv = uint32(b.readBitsNoRefill(24)) << uint(n-24)
		b.refill()
		rv += uint32(b.readBitsNoRefill(n - 24))
	}
	b.refill()
	return rv
}

func (b *bitReader) readMoreThan24BitsB(n int) uint32 {
	var rv uint32
	if n <= 24 {
		rv = uint32(b.readBitsNoRefillZero(n))
	} else {
		rv = uint32(b.readBitsNoRefill(24)) << uint(n-24)
		b.refillBackwards()
		rv += uint32(b.readBitsNoRefill(n - 24))
	}
	b.refillBackwards()
	return rv
}

func countLeadingZeros(x uint32) int {
	// 31 - BSR(x); for x==0 C is UB, callers guard x!=0.
	return bits.LeadingZeros32(x)
}

func (b *bitReader) readDistance(v uint32) uint32 {
	var w, m, n, rv uint32
	if v < 0xF0 {
		n = (v >> 4) + 4
		w = bits.RotateLeft32(b.bits|1, int(n))
		b.bitpos += int(n)
		m = (2 << n) - 1
		b.bits = w &^ m
		rv = ((w & m) << 4) + (v & 0xF) - 248
	} else {
		n = v - 0xF0 + 4
		w = bits.RotateLeft32(b.bits|1, int(n))
		b.bitpos += int(n)
		m = (2 << n) - 1
		b.bits = w &^ m
		rv = 8322816 + ((w & m) << 12)
		b.refill()
		rv += b.bits >> 20
		b.bitpos += 12
		b.bits <<= 12
	}
	b.refill()
	return rv
}

func (b *bitReader) readDistanceB(v uint32) uint32 {
	var w, m, n, rv uint32
	if v < 0xF0 {
		n = (v >> 4) + 4
		w = bits.RotateLeft32(b.bits|1, int(n))
		b.bitpos += int(n)
		m = (2 << n) - 1
		b.bits = w &^ m
		rv = ((w & m) << 4) + (v & 0xF) - 248
	} else {
		n = v - 0xF0 + 4
		w = bits.RotateLeft32(b.bits|1, int(n))
		b.bitpos += int(n)
		m = (2 << n) - 1
		b.bits = w &^ m
		rv = 8322816 + ((w & m) << 12)
		b.refillBackwards()
		rv += b.bits >> (32 - 12)
		b.bitpos += 12
		b.bits <<= 12
	}
	b.refillBackwards()
	return rv
}

func (b *bitReader) readLength() (uint32, bool) {
	n := bits.LeadingZeros32(b.bits) // 31 - BSR
	if n > 12 {
		return 0, false
	}
	b.bitpos += n
	b.bits <<= uint(n)
	b.refill()
	n += 7
	b.bitpos += n
	rv := (b.bits >> uint(32-n)) - 64
	b.bits <<= uint(n)
	b.refill()
	return rv, true
}

func (b *bitReader) readLengthB() (uint32, bool) {
	n := bits.LeadingZeros32(b.bits)
	if n > 12 {
		return 0, false
	}
	b.bitpos += n
	b.bits <<= uint(n)
	b.refillBackwards()
	n += 7
	b.bitpos += n
	rv := (b.bits >> uint(32-n)) - 64
	b.bits <<= uint(n)
	b.refillBackwards()
	return rv, true
}

// ---- block / quantum headers ----

type krakenHeader struct {
	decoderType    int
	restartDecoder bool
	uncompressed   bool
	useChecksums   bool
}

// parseHeader reads the 2-byte Kraken block header at buf[p:]. Returns new p, ok.
func (h *krakenHeader) parse(buf []byte, p int) (int, bool) {
	if p+2 > len(buf) {
		return p, false
	}
	b := int(buf[p])
	if (b & 0xF) == 0xC {
		if ((b >> 4) & 3) != 0 {
			return p, false
		}
		h.restartDecoder = (b>>7)&1 != 0
		h.uncompressed = (b>>6)&1 != 0
		b = int(buf[p+1])
		h.decoderType = b & 0x7F
		h.useChecksums = (b >> 7) != 0
		switch h.decoderType {
		case 6, 10, 5, 11, 12:
			return p + 2, true
		}
		return p, false
	}
	return p, false
}

type krakenQuantumHeader struct {
	compressedSize     uint32
	checksum           uint32
	flag1              uint8
	flag2              uint8
	wholeMatchDistance uint32
}

// parseQuantum reads a Kraken quantum header. Returns new p, ok.
func (h *krakenQuantumHeader) parse(buf []byte, p int, useChecksum bool) (int, bool) {
	if p+3 > len(buf) {
		return p, false
	}
	v := (uint32(buf[p]) << 16) | (uint32(buf[p+1]) << 8) | uint32(buf[p+2])
	size := v & 0x3FFFF
	if size != 0x3FFFF {
		h.compressedSize = size + 1
		h.flag1 = uint8((v >> 18) & 1)
		h.flag2 = uint8((v >> 19) & 1)
		if useChecksum {
			if p+6 > len(buf) {
				return p, false
			}
			h.checksum = (uint32(buf[p+3]) << 16) | (uint32(buf[p+4]) << 8) | uint32(buf[p+5])
			return p + 6, true
		}
		return p + 3, true
	}
	v >>= 18
	if v == 1 {
		// memset
		if p+4 > len(buf) {
			return p, false
		}
		h.checksum = uint32(buf[p+3])
		h.compressedSize = 0
		h.wholeMatchDistance = 0
		return p + 4, true
	}
	return p, false
}
