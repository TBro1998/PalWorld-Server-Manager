package oodle

import "math/bits"

// Huffman + Golomb-Rice entropy decoders. Ported from ooz kraken.cpp.

// ---- ReverseBitsArray2048 (kraken.cpp) ----
// A byte-level transpose implemented in the reference with SSE unpack
// intrinsics. Ported here scalar-for-scalar so the resulting permutation is
// byte-identical.

type v128 [16]byte

func vloadl(b []byte, off int) v128 {
	var r v128
	for k := 0; k < 8; k++ {
		if off+k >= 0 && off+k < len(b) {
			r[k] = b[off+k]
		}
	}
	return r
}

func vunpacklo(a, b v128) v128 {
	var r v128
	for k := 0; k < 8; k++ {
		r[2*k] = a[k]
		r[2*k+1] = b[k]
	}
	return r
}

func vunpackhi(a, b v128) v128 {
	var r v128
	for k := 0; k < 8; k++ {
		r[2*k] = a[k+8]
		r[2*k+1] = b[k+8]
	}
	return r
}

func vstorel(b []byte, off int, r v128) {
	for k := 0; k < 8; k++ {
		if off+k < len(b) {
			b[off+k] = r[k]
		}
	}
}

func vstoreh(b []byte, off int, r v128) {
	for k := 0; k < 8; k++ {
		if off+k < len(b) {
			b[off+k] = r[k+8]
		}
	}
}

// reverseBits2048 mirrors ReverseBitsArray2048. output is 2048(+16) bytes.
func reverseBits2048(input []byte) []byte {
	output := make([]byte, 2048+16)
	offsets := [32]byte{
		0, 0x80, 0x40, 0xC0, 0x20, 0xA0, 0x60, 0xE0, 0x10, 0x90, 0x50,
		0xD0, 0x30, 0xB0, 0x70, 0xF0, 0x08, 0x88, 0x48, 0xC8, 0x28, 0xA8,
		0x68, 0xE8, 0x18, 0x98, 0x58, 0xD8, 0x38, 0xB8, 0x78, 0xF8,
	}
	outp := 0
	for i := 0; i < 32; i++ {
		j := int(offsets[i])
		t0 := vunpacklo(vloadl(input, j), vloadl(input, j+256))
		t1 := vunpacklo(vloadl(input, j+512), vloadl(input, j+768))
		t2 := vunpacklo(vloadl(input, j+1024), vloadl(input, j+1280))
		t3 := vunpacklo(vloadl(input, j+1536), vloadl(input, j+1792))

		s0 := vunpacklo(t0, t1)
		s1 := vunpacklo(t2, t3)
		s2 := vunpackhi(t0, t1)
		s3 := vunpackhi(t2, t3)

		t0 = vunpacklo(s0, s1)
		t1 = vunpacklo(s2, s3)
		t2 = vunpackhi(s0, s1)
		t3 = vunpackhi(s2, s3)

		vstorel(output, outp+0, t0)
		vstoreh(output, outp+1024, t0)
		vstorel(output, outp+256, t1)
		vstoreh(output, outp+1280, t1)
		vstorel(output, outp+512, t2)
		vstoreh(output, outp+1536, t2)
		vstorel(output, outp+768, t3)
		vstoreh(output, outp+1792, t3)
		outp += 8
	}
	return output
}

func fillByte(dst []byte, pos int, v byte, n int) {
	for i := 0; i < n; i++ {
		if pos+i >= 0 && pos+i < len(dst) {
			dst[pos+i] = v
		}
	}
}

// ---- HuffReader / Kraken_DecodeBytesCore ----

// huffReader mirrors struct HuffReader. Positions are indices into buf (the
// current entropy block); out/outPos/outEnd address the decode target buffer.
type huffReader struct {
	out                                   []byte
	outPos, outEnd                        int
	buf                                   []byte
	src, srcMid, srcEnd, srcMidOrg        int
	srcBitpos, srcMidBitpos, srcEndBitpos int
	srcBits, srcMidBits, srcEndBits       uint32
}

// decodeBytesCore ports Kraken_DecodeBytesCore. bits2len/bits2sym are the
// reversed LUT arrays.
func decodeBytesCore(hr *huffReader, bits2len, bits2sym []byte) bool {
	buf := hr.buf
	src := hr.src
	srcBits := hr.srcBits
	srcBitpos := hr.srcBitpos

	srcMid := hr.srcMid
	srcMidBits := hr.srcMidBits
	srcMidBitpos := hr.srcMidBitpos

	srcEnd := hr.srcEnd
	srcEndBits := hr.srcEndBits
	srcEndBitpos := hr.srcEndBitpos

	var k int
	var n int

	dst := hr.outPos
	dstEnd := hr.outEnd
	out := hr.out

	if src > srcMid {
		return false
	}

	if hr.srcEnd-srcMid >= 4 && dstEnd-dst >= 6 {
		dstEnd -= 5
		srcEnd -= 4

		for dst < dstEnd && src <= srcMid && srcMid <= srcEnd {
			srcBits |= rd32(buf, src) << uint(srcBitpos)
			src += (31 - srcBitpos) >> 3

			srcEndBits |= bswap32(buf, srcEnd) << uint(srcEndBitpos)
			srcEnd -= (31 - srcEndBitpos) >> 3

			srcMidBits |= rd32(buf, srcMid) << uint(srcMidBitpos)
			srcMid += (31 - srcMidBitpos) >> 3

			srcBitpos |= 0x18
			srcEndBitpos |= 0x18
			srcMidBitpos |= 0x18

			k = int(srcBits & 0x7FF)
			n = int(bits2len[k])
			srcBits >>= uint(n)
			srcBitpos -= n
			out[dst+0] = bits2sym[k]

			k = int(srcEndBits & 0x7FF)
			n = int(bits2len[k])
			srcEndBits >>= uint(n)
			srcEndBitpos -= n
			out[dst+1] = bits2sym[k]

			k = int(srcMidBits & 0x7FF)
			n = int(bits2len[k])
			srcMidBits >>= uint(n)
			srcMidBitpos -= n
			out[dst+2] = bits2sym[k]

			k = int(srcBits & 0x7FF)
			n = int(bits2len[k])
			srcBits >>= uint(n)
			srcBitpos -= n
			out[dst+3] = bits2sym[k]

			k = int(srcEndBits & 0x7FF)
			n = int(bits2len[k])
			srcEndBits >>= uint(n)
			srcEndBitpos -= n
			out[dst+4] = bits2sym[k]

			k = int(srcMidBits & 0x7FF)
			n = int(bits2len[k])
			srcMidBits >>= uint(n)
			srcMidBitpos -= n
			out[dst+5] = bits2sym[k]
			dst += 6
		}
		dstEnd += 5

		src -= srcBitpos >> 3
		srcBitpos &= 7

		srcEnd += 4 + (srcEndBitpos >> 3)
		srcEndBitpos &= 7

		srcMid -= srcMidBitpos >> 3
		srcMidBitpos &= 7
	}

	for {
		if dst >= dstEnd {
			break
		}

		if srcMid-src <= 1 {
			if srcMid-src == 1 {
				srcBits |= bAt(buf, src) << uint(srcBitpos)
			}
		} else {
			srcBits |= rd16(buf, src) << uint(srcBitpos)
		}
		k = int(srcBits & 0x7FF)
		n = int(bits2len[k])
		srcBitpos -= n
		srcBits >>= uint(n)
		out[dst] = bits2sym[k]
		dst++
		src += (7 - srcBitpos) >> 3
		srcBitpos &= 7

		if dst < dstEnd {
			if srcEnd-srcMid <= 1 {
				if srcEnd-srcMid == 1 {
					srcEndBits |= bAt(buf, srcMid) << uint(srcEndBitpos)
					srcMidBits |= bAt(buf, srcMid) << uint(srcMidBitpos)
				}
			} else {
				v := rd16(buf, srcEnd-2)
				srcEndBits |= (((v >> 8) | (v << 8)) & 0xffff) << uint(srcEndBitpos)
				srcMidBits |= rd16(buf, srcMid) << uint(srcMidBitpos)
			}
			n = int(bits2len[srcEndBits&0x7FF])
			out[dst] = bits2sym[srcEndBits&0x7FF]
			dst++
			srcEndBitpos -= n
			srcEndBits >>= uint(n)
			srcEnd -= (7 - srcEndBitpos) >> 3
			srcEndBitpos &= 7
			if dst < dstEnd {
				n = int(bits2len[srcMidBits&0x7FF])
				out[dst] = bits2sym[srcMidBits&0x7FF]
				dst++
				srcMidBitpos -= n
				srcMidBits >>= uint(n)
				srcMid += (7 - srcMidBitpos) >> 3
				srcMidBitpos &= 7
			}
		}
		if src > srcMid || srcMid > srcEnd {
			return false
		}
	}
	if src != hr.srcMidOrg || srcEnd != srcMid {
		return false
	}
	return true
}

func bAt(b []byte, p int) uint32 {
	if p >= 0 && p < len(b) {
		return uint32(b[p])
	}
	return 0
}

// ---- Huff_ReadCodeLengthsOld ----

func readCodeLengthsOld(b *bitReader, syms []byte, codePrefix []uint32) int {
	if b.readBitNoRefill() != 0 {
		sym := 0
		numSymbols := 0
		avgBitsX4 := 32
		forcedBits := b.readBitsNoRefill(2)

		thres := uint32(1) << uint(31-(20>>uint(forcedBits)))
		var n, codelen int

		skip := b.readBit() != 0
		for {
			if !skip {
				if b.bits&0xff000000 == 0 {
					return -1
				}
				sym += b.readBitsNoRefill(2*(countLeadingZeros(b.bits)+1)) - 2 + 1
				if sym >= 256 {
					break
				}
			}
			skip = false
			b.refill()
			if b.bits&0xff000000 == 0 {
				return -1
			}
			n = b.readBitsNoRefill(2*(countLeadingZeros(b.bits)+1)) - 2 + 1
			if sym+n > 256 {
				return -1
			}
			b.refill()
			numSymbols += n
			for {
				if b.bits < thres {
					return -1
				}
				lz := countLeadingZeros(b.bits)
				v := b.readBitsNoRefill(lz+forcedBits+1) + ((lz - 1) << uint(forcedBits))
				codelen = ((-(v & 1)) ^ (v >> 1)) + ((avgBitsX4 + 2) >> 2)
				if codelen < 1 || codelen > 11 {
					return -1
				}
				avgBitsX4 = codelen + ((3*avgBitsX4 + 2) >> 2)
				b.refill()
				syms[codePrefix[codelen]] = byte(sym)
				codePrefix[codelen]++
				sym++
				n--
				if n == 0 {
					break
				}
			}
			if sym == 256 {
				break
			}
		}
		if sym == 256 && numSymbols >= 2 {
			return numSymbols
		}
		return -1
	}
	// Sparse symbol encoding
	numSymbols := b.readBitsNoRefill(8)
	if numSymbols == 0 {
		return -1
	}
	if numSymbols == 1 {
		syms[0] = byte(b.readBitsNoRefill(8))
	} else {
		codelenBits := b.readBitsNoRefill(3)
		if codelenBits > 4 {
			return -1
		}
		for i := 0; i < numSymbols; i++ {
			b.refill()
			sym := b.readBitsNoRefill(8)
			codelen := b.readBitsNoRefillZero(codelenBits) + 1
			if codelen > 11 {
				return -1
			}
			syms[codePrefix[codelen]] = byte(sym)
			codePrefix[codelen]++
		}
	}
	return numSymbols
}

// ---- BitReader_ReadFluff ----

func (b *bitReader) readFluff(numSymbols int) int {
	if numSymbols == 256 {
		return 0
	}
	x := 257 - numSymbols
	if x > numSymbols {
		x = numSymbols
	}
	x *= 2
	y := bsr(uint32(x-1)) + 1
	v := b.bits >> uint(32-y)
	z := uint32(1)<<uint(y) - uint32(x)
	if (v >> 1) >= z {
		b.bits <<= uint(y)
		b.bitpos += y
		return int(v - z)
	}
	b.bits <<= uint(y - 1)
	b.bitpos += y - 1
	return int(v >> 1)
}

// ---- Huff_ConvertToRanges ----

type huffRange struct {
	symbol uint16
	num    uint16
}

func convertToRanges(rng []huffRange, numSymbols, P int, symlen []byte, br *bitReader) int {
	numRanges := P >> 1
	symIdx := 0
	sp := 0

	if P&1 != 0 {
		br.refill()
		v := int(symlen[sp])
		sp++
		if v >= 8 {
			return -1
		}
		symIdx = br.readBitsNoRefill(v+1) + (1 << uint(v+1)) - 1
	}
	symsUsed := 0

	for i := 0; i < numRanges; i++ {
		br.refill()
		v := int(symlen[sp+0])
		if v >= 9 {
			return -1
		}
		num := br.readBitsNoRefillZero(v) + (1 << uint(v))
		v = int(symlen[sp+1])
		if v >= 8 {
			return -1
		}
		space := br.readBitsNoRefill(v+1) + (1 << uint(v+1)) - 1
		rng[i].symbol = uint16(symIdx)
		rng[i].num = uint16(num)
		symsUsed += num
		symIdx += num + space
		sp += 2
	}

	if symIdx >= 256 || symsUsed >= numSymbols || symIdx+numSymbols-symsUsed > 256 {
		return -1
	}

	rng[numRanges].symbol = uint16(symIdx)
	rng[numRanges].num = uint16(numSymbols - symsUsed)
	return numRanges + 1
}

// ---- Huff_ReadCodeLengthsNew ----

func readCodeLengthsNew(b *bitReader, syms []byte, codePrefix []uint32) int {
	forcedBits := b.readBitsNoRefill(2)
	numSymbols := b.readBitsNoRefill(8) + 1
	fluff := b.readFluff(numSymbols)

	var codeLen [512 + 16]byte
	var br2 bitReader2
	br2.bitpos = uint32((b.bitpos - 24) & 7)
	br2.buf = b.buf
	br2.pEnd = b.pEnd
	br2.p = b.p - ((24 - b.bitpos + 7) >> 3)

	if !decodeGolombRiceLengths(codeLen[:], numSymbols+fluff, &br2) {
		return -1
	}
	// codeLen[num+fluff : +16] already zero
	if !decodeGolombRiceBits(codeLen[:], numSymbols, forcedBits, &br2) {
		return -1
	}

	// Reset the bit decoder.
	b.bitpos = 24
	b.p = br2.p
	b.bits = 0
	b.refill()
	b.bits <<= br2.bitpos
	b.bitpos += int(br2.bitpos)

	runningSum := uint32(0x1e)
	for i := 0; i < numSymbols; i++ {
		v := int(codeLen[i])
		v = (-(v & 1)) ^ (v >> 1)
		codeLen[i] = byte(v + int(runningSum>>2) + 1)
		if codeLen[i] < 1 || codeLen[i] > 11 {
			return -1
		}
		runningSum += uint32(v)
	}

	var rng [128]huffRange
	ranges := convertToRanges(rng[:], numSymbols, fluff, codeLen[numSymbols:], b)
	if ranges <= 0 {
		return -1
	}

	cp := 0
	for i := 0; i < ranges; i++ {
		sym := int(rng[i].symbol)
		n := int(rng[i].num)
		for {
			syms[codePrefix[codeLen[cp]]] = byte(sym)
			codePrefix[codeLen[cp]]++
			cp++
			sym++
			n--
			if n == 0 {
				break
			}
		}
	}
	return numSymbols
}

// ---- Huff_MakeLut ----

func makeLut(prefixOrg, prefixCur []uint32, bits2len, bits2sym, syms []byte) bool {
	currslot := uint32(0)
	for i := uint32(1); i < 11; i++ {
		start := prefixOrg[i]
		count := prefixCur[i] - start
		if count != 0 {
			stepsize := uint32(1) << (11 - i)
			numToSet := count << (11 - i)
			if currslot+numToSet > 2048 {
				return false
			}
			fillByte(bits2len, int(currslot), byte(i), int(numToSet))
			p := int(currslot)
			for j := uint32(0); j != count; j++ {
				fillByte(bits2sym, p, syms[start+j], int(stepsize))
				p += int(stepsize)
			}
			currslot += numToSet
		}
	}
	if prefixCur[11]-prefixOrg[11] != 0 {
		numToSet := prefixCur[11] - prefixOrg[11]
		if currslot+numToSet > 2048 {
			return false
		}
		fillByte(bits2len, int(currslot), 11, int(numToSet))
		copy(bits2sym[currslot:currslot+numToSet], syms[prefixOrg[11]:prefixOrg[11]+numToSet])
		currslot += numToSet
	}
	return currslot == 2048
}

// ---- Kraken_DecodeBytes_Type12 ----

func decodeBytesType12(src []byte, srcSize int, output []byte, outputSize int, typ int) int {
	var br bitReader
	br.bitpos = 24
	br.bits = 0
	br.buf = src
	br.p = 0
	br.pEnd = srcSize
	br.refill()

	codePrefixOrg := [12]uint32{0x0, 0x0, 0x2, 0x6, 0xE, 0x1E, 0x3E, 0x7E, 0xFE, 0x1FE, 0x2FE, 0x3FE}
	codePrefix := [12]uint32{0x0, 0x0, 0x2, 0x6, 0xE, 0x1E, 0x3E, 0x7E, 0xFE, 0x1FE, 0x2FE, 0x3FE}
	var syms [1280]byte
	var numSyms int
	if br.readBitNoRefill() == 0 {
		numSyms = readCodeLengthsOld(&br, syms[:], codePrefix[:])
	} else if br.readBitNoRefill() == 0 {
		numSyms = readCodeLengthsNew(&br, syms[:], codePrefix[:])
	} else {
		return -1
	}

	if numSyms < 1 {
		return -1
	}
	srcPos := br.p - ((24 - br.bitpos) / 8)

	if numSyms == 1 {
		fillByte(output, 0, syms[0], outputSize)
		return srcPos - srcSize
	}

	var hufflen [2048 + 16]byte
	var hufsym [2048 + 16]byte
	if !makeLut(codePrefixOrg[:], codePrefix[:], hufflen[:], hufsym[:], syms[:]) {
		return -1
	}
	revlen := reverseBits2048(hufflen[:])
	revsym := reverseBits2048(hufsym[:])

	var hr huffReader
	if typ == 1 {
		if srcPos+3 > srcSize {
			return -1
		}
		splitMid := int(rd16(src, srcPos))
		srcPos += 2
		hr.out = output
		hr.outPos = 0
		hr.outEnd = outputSize
		hr.buf = src
		hr.src = srcPos
		hr.srcEnd = srcSize
		hr.srcMidOrg = srcPos + splitMid
		hr.srcMid = hr.srcMidOrg
		if !decodeBytesCore(&hr, revlen, revsym) {
			return -1
		}
	} else {
		if srcPos+6 > srcSize {
			return -1
		}
		halfOutputSize := (outputSize + 1) >> 1
		splitMid := int(rd32(src, srcPos) & 0xFFFFFF)
		srcPos += 3
		if splitMid > srcSize-srcPos {
			return -1
		}
		srcMidBlk := srcPos + splitMid
		splitLeft := int(rd16(src, srcPos))
		srcPos += 2
		if srcMidBlk-srcPos < splitLeft+2 || srcSize-srcMidBlk < 3 {
			return -1
		}
		splitRight := int(rd16(src, srcMidBlk))
		if srcSize-(srcMidBlk+2) < splitRight+2 {
			return -1
		}

		hr.out = output
		hr.outPos = 0
		hr.outEnd = halfOutputSize
		hr.buf = src
		hr.src = srcPos
		hr.srcEnd = srcMidBlk
		hr.srcMidOrg = srcPos + splitLeft
		hr.srcMid = hr.srcMidOrg
		if !decodeBytesCore(&hr, revlen, revsym) {
			return -1
		}

		hr = huffReader{}
		hr.out = output
		hr.outPos = halfOutputSize
		hr.outEnd = outputSize
		hr.buf = src
		hr.src = srcMidBlk + 2
		hr.srcEnd = srcSize
		hr.srcMidOrg = srcMidBlk + 2 + splitRight
		hr.srcMid = hr.srcMidOrg
		if !decodeBytesCore(&hr, revlen, revsym) {
			return -1
		}
	}
	return srcSize
}

// ---- BitReader2 + Golomb-Rice ----

type bitReader2 struct {
	buf    []byte
	p      int
	pEnd   int
	bitpos uint32
}

var kRiceCodeBits2Value = [256]uint32{
	0x80000000, 0x00000007, 0x10000006, 0x00000006, 0x20000005, 0x00000105, 0x10000005, 0x00000005, 0x30000004,
	0x00000204, 0x10000104, 0x00000104, 0x20000004, 0x00010004, 0x10000004, 0x00000004, 0x40000003, 0x00000303,
	0x10000203, 0x00000203, 0x20000103, 0x00010103, 0x10000103, 0x00000103, 0x30000003, 0x00020003, 0x10010003,
	0x00010003, 0x20000003, 0x01000003, 0x10000003, 0x00000003, 0x50000002, 0x00000402, 0x10000302, 0x00000302,
	0x20000202, 0x00010202, 0x10000202, 0x00000202, 0x30000102, 0x00020102, 0x10010102, 0x00010102, 0x20000102,
	0x01000102, 0x10000102, 0x00000102, 0x40000002, 0x00030002, 0x10020002, 0x00020002, 0x20010002, 0x01010002,
	0x10010002, 0x00010002, 0x30000002, 0x02000002, 0x11000002, 0x01000002, 0x20000002, 0x00000012, 0x10000002,
	0x00000002, 0x60000001, 0x00000501, 0x10000401, 0x00000401, 0x20000301, 0x00010301, 0x10000301, 0x00000301,
	0x30000201, 0x00020201, 0x10010201, 0x00010201, 0x20000201, 0x01000201, 0x10000201, 0x00000201, 0x40000101,
	0x00030101, 0x10020101, 0x00020101, 0x20010101, 0x01010101, 0x10010101, 0x00010101, 0x30000101, 0x02000101,
	0x11000101, 0x01000101, 0x20000101, 0x00000111, 0x10000101, 0x00000101, 0x50000001, 0x00040001, 0x10030001,
	0x00030001, 0x20020001, 0x01020001, 0x10020001, 0x00020001, 0x30010001, 0x02010001, 0x11010001, 0x01010001,
	0x20010001, 0x00010011, 0x10010001, 0x00010001, 0x40000001, 0x03000001, 0x12000001, 0x02000001, 0x21000001,
	0x01000011, 0x11000001, 0x01000001, 0x30000001, 0x00000021, 0x10000011, 0x00000011, 0x20000001, 0x00001001,
	0x10000001, 0x00000001, 0x70000000, 0x00000600, 0x10000500, 0x00000500, 0x20000400, 0x00010400, 0x10000400,
	0x00000400, 0x30000300, 0x00020300, 0x10010300, 0x00010300, 0x20000300, 0x01000300, 0x10000300, 0x00000300,
	0x40000200, 0x00030200, 0x10020200, 0x00020200, 0x20010200, 0x01010200, 0x10010200, 0x00010200, 0x30000200,
	0x02000200, 0x11000200, 0x01000200, 0x20000200, 0x00000210, 0x10000200, 0x00000200, 0x50000100, 0x00040100,
	0x10030100, 0x00030100, 0x20020100, 0x01020100, 0x10020100, 0x00020100, 0x30010100, 0x02010100, 0x11010100,
	0x01010100, 0x20010100, 0x00010110, 0x10010100, 0x00010100, 0x40000100, 0x03000100, 0x12000100, 0x02000100,
	0x21000100, 0x01000110, 0x11000100, 0x01000100, 0x30000100, 0x00000120, 0x10000110, 0x00000110, 0x20000100,
	0x00001100, 0x10000100, 0x00000100, 0x60000000, 0x00050000, 0x10040000, 0x00040000, 0x20030000, 0x01030000,
	0x10030000, 0x00030000, 0x30020000, 0x02020000, 0x11020000, 0x01020000, 0x20020000, 0x00020010, 0x10020000,
	0x00020000, 0x40010000, 0x03010000, 0x12010000, 0x02010000, 0x21010000, 0x01010010, 0x11010000, 0x01010000,
	0x30010000, 0x00010020, 0x10010010, 0x00010010, 0x20010000, 0x00011000, 0x10010000, 0x00010000, 0x50000000,
	0x04000000, 0x13000000, 0x03000000, 0x22000000, 0x02000010, 0x12000000, 0x02000000, 0x31000000, 0x01000020,
	0x11000010, 0x01000010, 0x21000000, 0x01001000, 0x11000000, 0x01000000, 0x40000000, 0x00000030, 0x10000020,
	0x00000020, 0x20000010, 0x00001010, 0x10000010, 0x00000010, 0x30000000, 0x00002000, 0x10001000, 0x00001000,
	0x20000000, 0x00100000, 0x10000000, 0x00000000,
}

var kRiceCodeBits2Len = [256]uint8{
	0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, 2, 3, 3, 4, 1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5, 1, 2, 2, 3, 2,
	3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5, 2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6, 1, 2, 2, 3, 2, 3, 3, 4, 2, 3,
	3, 4, 3, 4, 4, 5, 2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6, 2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5,
	6, 3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7, 1, 2, 2, 3, 2, 3, 3, 4, 2, 3, 3, 4, 3, 4, 4, 5, 2, 3, 3, 4,
	3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6, 2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6, 3, 4, 4, 5, 4, 5, 5, 6, 4,
	5, 5, 6, 5, 6, 6, 7, 2, 3, 3, 4, 3, 4, 4, 5, 3, 4, 4, 5, 4, 5, 5, 6, 3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6,
	6, 7, 3, 4, 4, 5, 4, 5, 5, 6, 4, 5, 5, 6, 5, 6, 6, 7, 4, 5, 5, 6, 5, 6, 6, 7, 5, 6, 6, 7, 6, 7, 7, 8,
}

// decodeGolombRiceLengths ports DecodeGolombRiceLengths.
func decodeGolombRiceLengths(dst []byte, size int, br *bitReader2) bool {
	p := br.p
	pEnd := br.pEnd
	dstPos := 0
	dstEnd := size
	if p >= pEnd {
		return false
	}

	count := -int(br.bitpos)
	v := uint32(br.buf[p]) & (255 >> br.bitpos)
	p++
	for {
		if v == 0 {
			count += 8
		} else {
			x := kRiceCodeBits2Value[v]
			st32(dst, dstPos+0, uint32(count)+(x&0x0f0f0f0f))
			st32(dst, dstPos+4, (x>>4)&0x0f0f0f0f)
			dstPos += int(kRiceCodeBits2Len[v])
			if dstPos >= dstEnd {
				break
			}
			count = int(x >> 28)
		}
		if p >= pEnd {
			return false
		}
		v = uint32(br.buf[p])
		p++
	}
	// went too far, step back
	if dstPos > dstEnd {
		n := dstPos - dstEnd
		for ; n > 0; n-- {
			v &= v - 1
		}
	}
	// step back if byte not finished
	bitpos := 0
	if v&1 == 0 {
		p--
		q := bsf(v)
		bitpos = 8 - q
	}
	br.p = p
	br.bitpos = uint32(bitpos)
	return true
}

// decodeGolombRiceBits ports DecodeGolombRiceBits.
func decodeGolombRiceBits(dst []byte, size int, bitcount int, br *bitReader2) bool {
	if bitcount == 0 {
		return true
	}
	dstEnd := size
	p := br.p
	bitpos := int(br.bitpos)

	bitsRequired := bitpos + bitcount*size
	bytesRequired := (bitsRequired + 7) >> 3
	if bytesRequired > br.pEnd-p {
		return false
	}

	br.p = p + (bitsRequired >> 3)
	br.bitpos = uint32(bitsRequired & 7)

	bak := rd64(dst, dstEnd)
	dstPos := 0

	switch {
	case bitcount < 2:
		// bitcount == 1
		for dstPos < dstEnd {
			bt := uint64(uint8(bswap32(br.buf, p) >> uint(24-bitpos)))
			p++
			bt = (bt | (bt << 28)) & 0xF0000000F
			bt = (bt | (bt << 14)) & 0x3000300030003
			bt = (bt | (bt << 7)) & 0x0101010101010101
			st64(dst, dstPos, rd64(dst, dstPos)*2+bits.ReverseBytes64(bt))
			dstPos += 8
		}
	case bitcount == 2:
		for dstPos < dstEnd {
			bt := uint64(uint16(bswap32(br.buf, p) >> uint(16-bitpos)))
			p += 2
			bt = (bt | (bt << 24)) & 0xFF000000FF
			bt = (bt | (bt << 12)) & 0xF000F000F000F
			bt = (bt | (bt << 6)) & 0x0303030303030303
			st64(dst, dstPos, rd64(dst, dstPos)*4+bits.ReverseBytes64(bt))
			dstPos += 8
		}
	default:
		// bitcount == 3
		for dstPos < dstEnd {
			bt := uint64(bswap32(br.buf, p)>>uint(8-bitpos)) & 0xffffff
			p += 3
			bt = (bt | (bt << 20)) & 0xFFF00000FFF
			bt = (bt | (bt << 10)) & 0x3F003F003F003F
			bt = (bt | (bt << 5)) & 0x0707070707070707
			st64(dst, dstPos, rd64(dst, dstPos)*8+bits.ReverseBytes64(bt))
			dstPos += 8
		}
	}
	st64(dst, dstEnd, bak)
	return true
}
