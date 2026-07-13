package oodle

import "math/bits"

// TANS (tabled asymmetric numeral system) decoder. Ported from ooz kraken.cpp.

type tansData struct {
	aUsed uint32
	bUsed uint32
	a     [256]uint8
	b     [256]uint32
}

// simpleSortU8 / simpleSortU32 port the template SimpleSort (insertion sort).
func simpleSortU8(p []uint8) {
	for i := 1; i < len(p); i++ {
		t := p[i]
		j := i
		for j > 0 && t < p[j-1] {
			p[j] = p[j-1]
			j--
		}
		p[j] = t
	}
}

func simpleSortU32(p []uint32) {
	for i := 1; i < len(p); i++ {
		t := p[i]
		j := i
		for j > 0 && t < p[j-1] {
			p[j] = p[j-1]
			j--
		}
		p[j] = t
	}
}

// tansDecodeTable ports Tans_DecodeTable.
func tansDecodeTable(b *bitReader, lBits int, td *tansData) bool {
	b.refill()
	if b.readBitNoRefill() != 0 {
		Q := b.readBitsNoRefill(3)
		numSymbols := b.readBitsNoRefill(8) + 1
		if numSymbols < 2 {
			return false
		}
		fluff := b.readFluff(numSymbols)
		totalRiceValues := fluff + numSymbols
		var rice [512 + 16]uint8
		var br2 bitReader2

		br2.p = b.p - int(uint32(24-b.bitpos+7)>>3)
		br2.pEnd = b.pEnd
		br2.buf = b.buf
		br2.bitpos = uint32((b.bitpos - 24) & 7)

		if !decodeGolombRiceLengths(rice[:], totalRiceValues, &br2) {
			return false
		}
		// rice[totalRiceValues:+16] already zero

		b.bitpos = 24
		b.p = br2.p
		b.bits = 0
		b.refill()
		b.bits <<= br2.bitpos
		b.bitpos += int(br2.bitpos)

		var rng [133]huffRange
		fluff = convertToRanges(rng[:], numSymbols, fluff, rice[numSymbols:], b)
		if fluff < 0 {
			return false
		}

		b.refill()

		L := uint32(1) << uint(lBits)
		curRicePtr := 0
		average := 6
		somesum := 0
		aIdx := 0
		bIdx := 0

		for ri := 0; ri < fluff; ri++ {
			symbol := int(rng[ri].symbol)
			num := int(rng[ri].num)
			for {
				b.refill()

				nextra := Q + int(rice[curRicePtr])
				curRicePtr++
				if nextra > 15 {
					return false
				}
				v := b.readBitsNoRefillZero(nextra) + (1 << uint(nextra)) - (1 << uint(Q))

				averageDiv4 := average >> 2
				limit := 2 * averageDiv4
				if v <= limit {
					v = averageDiv4 + ((-(v & 1)) ^ (int(uint32(v) >> 1)))
				}
				if limit > v {
					limit = v
				}
				v += 1
				average += limit - averageDiv4
				td.a[aIdx] = uint8(symbol)
				td.b[bIdx] = uint32(symbol<<16) + uint32(v)
				if v == 1 {
					aIdx++
				}
				if v >= 2 {
					bIdx++
				}
				somesum += v
				symbol++
				num--
				if num == 0 {
					break
				}
			}
		}
		td.aUsed = uint32(aIdx)
		td.bUsed = uint32(bIdx)
		if somesum != int(L) {
			return false
		}
		return true
	}

	var seen [256]bool
	L := uint32(1) << uint(lBits)

	count := b.readBitsNoRefill(3) + 1

	bitsPerSym := bsr(uint32(lBits)) + 1
	maxDeltaBits := b.readBitsNoRefill(bitsPerSym)

	if maxDeltaBits == 0 || maxDeltaBits > lBits {
		return false
	}

	aIdx := 0
	bIdx := 0

	weight := 0
	totalWeights := 0

	for {
		b.refill()

		sym := b.readBitsNoRefill(8)
		if seen[sym] {
			return false
		}

		delta := b.readBitsNoRefill(maxDeltaBits)

		weight += delta

		if weight == 0 {
			return false
		}

		seen[sym] = true
		if weight == 1 {
			td.a[aIdx] = uint8(sym)
			aIdx++
		} else {
			td.b[bIdx] = uint32(sym<<16) + uint32(weight)
			bIdx++
		}

		totalWeights += weight

		count--
		if count == 0 {
			break
		}
	}

	b.refill()

	sym := b.readBitsNoRefill(8)
	if seen[sym] {
		return false
	}

	if int(L)-totalWeights < weight || int(L)-totalWeights <= 1 {
		return false
	}

	td.b[bIdx] = uint32(sym<<16) + uint32(int(L)-totalWeights)
	bIdx++

	td.aUsed = uint32(aIdx)
	td.bUsed = uint32(bIdx)

	simpleSortU8(td.a[:aIdx])
	simpleSortU32(td.b[:bIdx])
	return true
}

type tansLutEnt struct {
	x      uint32
	bitsX  uint8
	symbol uint8
	w      uint16
}

// tansInitLut ports Tans_InitLut.
func tansInitLut(td *tansData, lBits int, lut []tansLutEnt) {
	var pointers [4]int

	L := 1 << uint(lBits)
	aUsed := int(td.aUsed)

	slotsLeft := L - aUsed

	sa := slotsLeft >> 2
	pointers[0] = 0
	sb := sa + b2i((slotsLeft&3) > 0)
	pointers[1] = sb
	sb += sa + b2i((slotsLeft&3) > 1)
	pointers[2] = sb
	sb += sa + b2i((slotsLeft&3) > 2)
	pointers[3] = sb

	// Setup single entries with weight=1
	{
		base := slotsLeft
		var le tansLutEnt
		le.w = 0
		le.bitsX = uint8(lBits)
		le.x = uint32(1<<uint(lBits)) - 1
		for i := 0; i < aUsed; i++ {
			lut[base+i] = le
			lut[base+i].symbol = td.a[i]
		}
	}

	// Setup entries with weight >= 2
	weightsSum := 0
	for i := 0; i < int(td.bUsed); i++ {
		weight := int(td.b[i] & 0xffff)
		symbol := int(td.b[i] >> 16)
		if weight > 4 {
			symBits := bsr(uint32(weight))
			Z := lBits - symBits
			var le tansLutEnt
			le.symbol = uint8(symbol)
			le.bitsX = uint8(Z)
			le.x = uint32(1<<uint(Z)) - 1
			le.w = uint16((L - 1) & (weight << uint(Z)))
			whatToAdd := 1 << uint(Z)
			X := (1 << uint(symBits+1)) - weight

			for j := 0; j < 4; j++ {
				dst := pointers[j]

				Y := (weight + ((weightsSum - j - 1) & 3)) >> 2
				if X >= Y {
					for n := Y; n > 0; n-- {
						lut[dst] = le
						dst++
						le.w += uint16(whatToAdd)
					}
					X -= Y
				} else {
					for n := X; n > 0; n-- {
						lut[dst] = le
						dst++
						le.w += uint16(whatToAdd)
					}
					Z--

					whatToAdd >>= 1
					le.bitsX = uint8(Z)
					le.w = 0
					le.x >>= 1
					for n := Y - X; n > 0; n-- {
						lut[dst] = le
						dst++
						le.w += uint16(whatToAdd)
					}
					X = weight
				}
				pointers[j] = dst
			}
		} else {
			bitsv := uint32(((1 << uint(weight)) - 1) << uint(weightsSum&3))
			bitsv |= bitsv >> 4
			n := weight
			ww := weight
			for {
				idx := bsf(bitsv)
				bitsv &= bitsv - 1
				dst := pointers[idx]
				pointers[idx]++
				lut[dst].symbol = uint8(symbol)
				weightBits := bsr(uint32(ww))
				lut[dst].bitsX = uint8(lBits - weightBits)
				lut[dst].x = uint32(1<<uint(lBits-weightBits)) - 1
				lut[dst].w = uint16((L - 1) & (ww << uint(lBits-weightBits)))
				ww++
				n--
				if n == 0 {
					break
				}
			}
		}
		weightsSum += weight
	}
}

func b2i(v bool) int {
	if v {
		return 1
	}
	return 0
}

type tansDecoderParams struct {
	lut                                    []tansLutEnt
	dst                                    []byte
	dstPos                                 int
	dstEnd                                 int
	buf                                    []byte
	ptrF, ptrB                             int
	bitsF, bitsB                           uint32
	bitposF                                int
	bitposB                                int
	state0, state1, state2, state3, state4 uint32
}

// tansDecode ports Tans_Decode.
func tansDecode(p *tansDecoderParams) bool {
	lut := p.lut
	dst := p.dst
	dstPos := p.dstPos
	dstEnd := p.dstEnd
	buf := p.buf
	ptrF := p.ptrF
	ptrB := p.ptrB
	bitsF := p.bitsF
	bitsB := p.bitsB
	bitposF := p.bitposF
	bitposB := p.bitposB
	state0 := p.state0
	state1 := p.state1
	state2 := p.state2
	state3 := p.state3
	state4 := p.state4

	if ptrF > ptrB {
		return false
	}

	var e *tansLutEnt

	forwardBits := func() {
		bitsF |= rd32(buf, ptrF) << uint(bitposF)
		ptrF += (31 - bitposF) >> 3
		bitposF |= 24
	}
	backwardBits := func() {
		bitsB |= bswap32(buf, ptrB-4) << uint(bitposB)
		ptrB -= (31 - bitposB) >> 3
		bitposB |= 24
	}

	if dstPos < dstEnd {
		for {
			forwardBits()
			// round state0
			e = &lut[state0]
			dst[dstPos] = e.symbol
			dstPos++
			bitposF -= int(e.bitsX)
			state0 = (bitsF & e.x) + uint32(e.w)
			bitsF >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			// round state1
			e = &lut[state1]
			dst[dstPos] = e.symbol
			dstPos++
			bitposF -= int(e.bitsX)
			state1 = (bitsF & e.x) + uint32(e.w)
			bitsF >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			forwardBits()
			// round state2
			e = &lut[state2]
			dst[dstPos] = e.symbol
			dstPos++
			bitposF -= int(e.bitsX)
			state2 = (bitsF & e.x) + uint32(e.w)
			bitsF >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			// round state3
			e = &lut[state3]
			dst[dstPos] = e.symbol
			dstPos++
			bitposF -= int(e.bitsX)
			state3 = (bitsF & e.x) + uint32(e.w)
			bitsF >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			forwardBits()
			// round state4
			e = &lut[state4]
			dst[dstPos] = e.symbol
			dstPos++
			bitposF -= int(e.bitsX)
			state4 = (bitsF & e.x) + uint32(e.w)
			bitsF >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			backwardBits()
			// round state0
			e = &lut[state0]
			dst[dstPos] = e.symbol
			dstPos++
			bitposB -= int(e.bitsX)
			state0 = (bitsB & e.x) + uint32(e.w)
			bitsB >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			// round state1
			e = &lut[state1]
			dst[dstPos] = e.symbol
			dstPos++
			bitposB -= int(e.bitsX)
			state1 = (bitsB & e.x) + uint32(e.w)
			bitsB >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			backwardBits()
			// round state2
			e = &lut[state2]
			dst[dstPos] = e.symbol
			dstPos++
			bitposB -= int(e.bitsX)
			state2 = (bitsB & e.x) + uint32(e.w)
			bitsB >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			// round state3
			e = &lut[state3]
			dst[dstPos] = e.symbol
			dstPos++
			bitposB -= int(e.bitsX)
			state3 = (bitsB & e.x) + uint32(e.w)
			bitsB >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
			backwardBits()
			// round state4
			e = &lut[state4]
			dst[dstPos] = e.symbol
			dstPos++
			bitposB -= int(e.bitsX)
			state4 = (bitsB & e.x) + uint32(e.w)
			bitsB >>= e.bitsX
			if dstPos >= dstEnd {
				break
			}
		}
	}

	if ptrB-ptrF+(bitposF>>3)+(bitposB>>3) != 0 {
		return false
	}

	statesOr := state0 | state1 | state2 | state3 | state4
	if statesOr & ^uint32(0xFF) != 0 {
		return false
	}

	dst[dstEnd+0] = uint8(state0)
	dst[dstEnd+1] = uint8(state1)
	dst[dstEnd+2] = uint8(state2)
	dst[dstEnd+3] = uint8(state3)
	dst[dstEnd+4] = uint8(state4)
	return true
}

// krakDecodeTans ports Krak_DecodeTans. src is the block; dst is the output
// slice (must have >=5 bytes of padding past dstSize).
func krakDecodeTans(src []byte, srcSize int, dst []byte, dstSize int, scratch []byte) int {
	if srcSize < 8 || dstSize < 5 {
		return -1
	}

	var br bitReader
	br.bitpos = 24
	br.bits = 0
	br.buf = src
	br.p = 0
	br.pEnd = srcSize
	br.refill()

	// reserved bit
	if br.readBitNoRefill() != 0 {
		return -1
	}

	lBits := br.readBitsNoRefill(2) + 8

	var td tansData
	if !tansDecodeTable(&br, lBits, &td) {
		return -1
	}

	srcPos := br.p - (24-br.bitpos)/8

	if srcPos >= srcSize {
		return -1
	}

	lut := make([]tansLutEnt, 1<<uint(lBits))
	tansInitLut(&td, lBits, lut)
	_ = scratch

	var params tansDecoderParams
	params.dst = dst
	params.dstPos = 0
	params.dstEnd = dstSize - 5
	params.buf = src
	params.lut = lut

	// Read out the initial state
	lMask := uint32(1<<uint(lBits)) - 1
	bitsF := rd32(src, srcPos)
	srcPos += 4
	bitsB := bswap32(src, srcSize-4)
	srcEnd := srcSize - 4
	bitposF := 32
	bitposB := 32

	params.state0 = bitsF & lMask
	params.state1 = bitsB & lMask
	bitsF >>= uint(lBits)
	bitposF -= lBits
	bitsB >>= uint(lBits)
	bitposB -= lBits

	params.state2 = bitsF & lMask
	params.state3 = bitsB & lMask
	bitsF >>= uint(lBits)
	bitposF -= lBits
	bitsB >>= uint(lBits)
	bitposB -= lBits

	bitsF |= rd32(src, srcPos) << uint(bitposF)
	srcPos += (31 - bitposF) >> 3
	bitposF |= 24

	params.state4 = bitsF & lMask
	bitsF >>= uint(lBits)
	bitposF -= lBits

	params.bitsF = bitsF
	params.ptrF = srcPos - (bitposF >> 3)
	params.bitposF = bitposF & 7

	params.bitsB = bitsB
	params.ptrB = srcEnd + (bitposB >> 3)
	params.bitposB = bitposB & 7

	if !tansDecode(&params) {
		return -1
	}

	return srcSize
}

var _ = bits.ReverseBytes32
