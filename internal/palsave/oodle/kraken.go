package oodle

import (
	"errors"
	"fmt"
)

// Kraken LZ table + quantum decode + top-level loop. Ported from ooz kraken.cpp.

const (
	scratchSize = 0x6C000
	safePad     = 64
)

// krakenLzTable mirrors struct KrakenLzTable. The packed byte streams live in
// scratch (or alias src for stored chunks); the unpacked offset/length streams
// are their own int32 slices.
type krakenLzTable struct {
	cmdStream      []byte
	cmdStreamSize  int
	offsStream     []int32
	offsStreamSize int
	litStream      []byte
	litStreamSize  int
	lenStream      []int32
	lenStreamSize  int
}

type krakenDecoder struct {
	scratch []byte
	hdr     krakenHeader
}

// combineScaledOffsetArrays ports CombineScaledOffsetArrays.
func combineScaledOffsetArrays(offs []int32, scale int, lowBits []byte) {
	for i := 0; i < len(offs); i++ {
		offs[i] = int32(scale)*offs[i] - int32(lowBits[i])
	}
}

// krakenUnpackOffsets ports Kraken_UnpackOffsets. buf is the [src, src_end]
// region holding the packed varbit streams.
func krakenUnpackOffsets(buf []byte, packedOffs, packedOffsExtra []byte, packedOffsSize, multiDistScale int,
	packedLitlen []byte, packedLitlenSize int, offsStream, lenStream []int32, excessFlag bool, excessBytes int) bool {

	var bitsA, bitsB bitReader
	bitsA.bitpos = 24
	bitsA.bits = 0
	bitsA.buf = buf
	bitsA.p = 0
	bitsA.pEnd = len(buf)
	bitsA.refill()

	bitsB.bitpos = 24
	bitsB.bits = 0
	bitsB.buf = buf
	bitsB.p = len(buf)
	bitsB.pEnd = 0
	bitsB.refillBackwards()

	u32LenStreamSize := 0
	if !excessFlag {
		if bitsB.bits < 0x2000 {
			return false
		}
		n := 31 - bsr(bitsB.bits)
		bitsB.bitpos += n
		bitsB.bits <<= uint(n)
		bitsB.refillBackwards()
		n++
		u32LenStreamSize = int(bitsB.bits>>uint(32-n)) - 1
		bitsB.bitpos += n
		bitsB.bits <<= uint(n)
		bitsB.refillBackwards()
	}

	oi := 0
	if multiDistScale == 0 {
		pi := 0
		for pi != packedOffsSize {
			offsStream[oi] = -int32(bitsA.readDistance(uint32(packedOffs[pi])))
			pi++
			oi++
			if pi == packedOffsSize {
				break
			}
			offsStream[oi] = -int32(bitsB.readDistanceB(uint32(packedOffs[pi])))
			pi++
			oi++
		}
	} else {
		oiOrg := oi
		pi := 0
		for pi != packedOffsSize {
			cmd := uint32(packedOffs[pi])
			pi++
			if (cmd >> 3) > 26 {
				return false
			}
			offs := ((8 + (cmd & 7)) << (cmd >> 3)) | bitsA.readMoreThan24Bits(int(cmd>>3))
			offsStream[oi] = 8 - int32(offs)
			oi++
			if pi == packedOffsSize {
				break
			}
			cmd = uint32(packedOffs[pi])
			pi++
			if (cmd >> 3) > 26 {
				return false
			}
			offs = ((8 + (cmd & 7)) << (cmd >> 3)) | bitsB.readMoreThan24BitsB(int(cmd>>3))
			offsStream[oi] = 8 - int32(offs)
			oi++
		}
		if multiDistScale != 1 {
			combineScaledOffsetArrays(offsStream[oiOrg:oi], multiDistScale, packedOffsExtra)
		}
	}

	var u32LenStream [512]uint32
	if u32LenStreamSize > 512 {
		return false
	}
	ui := 0
	i := 0
	for ; i+1 < u32LenStreamSize; i += 2 {
		v, ok := bitsA.readLength()
		if !ok {
			return false
		}
		u32LenStream[i+0] = v
		v2, ok2 := bitsB.readLengthB()
		if !ok2 {
			return false
		}
		u32LenStream[i+1] = v2
	}
	if i < u32LenStreamSize {
		v, ok := bitsA.readLength()
		if !ok {
			return false
		}
		u32LenStream[i+0] = v
	}

	bitsA.p -= (24 - bitsA.bitpos) >> 3
	bitsB.p += (24 - bitsB.bitpos) >> 3

	if bitsA.p != bitsB.p {
		return false
	}

	for i := 0; i < packedLitlenSize; i++ {
		v := uint32(packedLitlen[i])
		if v == 255 {
			v = u32LenStream[ui] + 255
			ui++
		}
		lenStream[i] = int32(v + 3)
	}
	if ui != u32LenStreamSize {
		return false
	}
	return true
}

// krakenReadLzTable ports Kraken_ReadLzTable. out is the full destination
// buffer; offset is the current absolute write position (dst - dst_start).
func krakenReadLzTable(mode int, block, out []byte, offset, dstSize int, scratch []byte, lz *krakenLzTable) bool {
	if mode > 1 {
		return false
	}
	srcEnd := len(block)
	srcPos := 0
	if srcEnd-srcPos < 13 {
		return false
	}

	dstPos := offset
	if offset == 0 {
		copy(out[dstPos:dstPos+8], block[srcPos:srcPos+8])
		dstPos += 8
		srcPos += 8
	}
	_ = dstPos

	if block[srcPos]&0x80 != 0 {
		flag := block[srcPos]
		srcPos++
		if flag&0xc0 != 0x80 {
			return false // reserved flag set
		}
		return false // excess bytes not supported
	}

	forceCopy := false // src and dst are distinct buffers; never overlap

	scr := scratch

	// Decode lit stream, bounded by dst_size.
	outLit, dcLit, n := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), dstSize), forceCopy, scr)
	if n < 0 {
		return false
	}
	srcPos += n
	lz.litStream = outLit
	lz.litStreamSize = dcLit
	scr = scr[dcLit:]

	// Decode command stream, bounded by dst_size.
	outCmd, dcCmd, n := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), dstSize), forceCopy, scr)
	if n < 0 {
		return false
	}
	srcPos += n
	lz.cmdStream = outCmd
	lz.cmdStreamSize = dcCmd
	scr = scr[dcCmd:]

	if srcEnd-srcPos < 3 {
		return false
	}

	offsScaling := 0
	var packedOffsExtra []byte
	var packedOffs []byte
	offsStreamSize := 0

	if block[srcPos]&0x80 != 0 {
		// distances coded with 2 tables
		offsScaling = int(block[srcPos]) - 127
		srcPos++

		po, oss, nn := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), lz.cmdStreamSize), false, scr)
		if nn < 0 {
			return false
		}
		srcPos += nn
		packedOffs = po
		offsStreamSize = oss
		scr = scr[oss:]

		if offsScaling != 1 {
			poe, dc, nn2 := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), offsStreamSize), false, scr)
			if nn2 < 0 || dc != offsStreamSize {
				return false
			}
			srcPos += nn2
			packedOffsExtra = poe
			scr = scr[dc:]
		}
	} else {
		po, oss, nn := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), lz.cmdStreamSize), false, scr)
		if nn < 0 {
			return false
		}
		srcPos += nn
		packedOffs = po
		offsStreamSize = oss
		scr = scr[oss:]
	}
	lz.offsStreamSize = offsStreamSize

	// Decode packed litlen stream. Bounded by 1/4 of dst_size.
	packedLen, lss, n := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), dstSize>>2), false, scr)
	if n < 0 {
		return false
	}
	srcPos += n
	lz.lenStreamSize = lss
	scr = scr[lss:]
	_ = scr

	// Final unpacked streams (padded so the C peek-one-past reads stay in range).
	lz.offsStream = make([]int32, offsStreamSize+8)
	lz.lenStream = make([]int32, lss+8)

	buf := block[srcPos:srcEnd]
	return krakenUnpackOffsets(buf, packedOffs, packedOffsExtra, offsStreamSize, offsScaling,
		packedLen, lss, lz.offsStream, lz.lenStream, false, 0)
}

// matchCopy copies length bytes forward within out, byte-by-byte, which is the
// correct semantics for overlapping LZ matches (offset < length).
func matchCopy(out []byte, dst, src, length int) {
	for i := 0; i < length; i++ {
		out[dst+i] = out[src+i]
	}
}

// processLzRunsType1 ports Kraken_ProcessLzRuns_Type1 (raw literals).
func processLzRunsType1(lz *krakenLzTable, out []byte, dst, dstEnd, dstStart int) bool {
	cmd := lz.cmdStream
	cmdPos, cmdEnd := 0, lz.cmdStreamSize
	lenStream := lz.lenStream
	lenPos, lenEnd := 0, lz.lenStreamSize
	lit := lz.litStream
	litPos, litEnd := 0, lz.litStreamSize
	offs := lz.offsStream
	offsPos, offsEnd := 0, lz.offsStreamSize

	var recentOffs [7]int32
	recentOffs[3] = -8
	recentOffs[4] = -8
	recentOffs[5] = -8

	for cmdPos < cmdEnd {
		f := uint32(cmd[cmdPos])
		cmdPos++
		litlen := int(f & 3)
		offsIndex := int(f >> 6)
		matchlen := int((f >> 2) & 0xF)

		nextLongLength := int(lenStream[lenPos])
		if litlen == 3 {
			litlen = nextLongLength
			lenPos++
		}
		recentOffs[6] = offs[offsPos]

		copy(out[dst:dst+litlen], lit[litPos:litPos+litlen])
		dst += litlen
		litPos += litlen

		offset := int(recentOffs[offsIndex+3])
		recentOffs[offsIndex+3] = recentOffs[offsIndex+2]
		recentOffs[offsIndex+2] = recentOffs[offsIndex+1]
		recentOffs[offsIndex+1] = recentOffs[offsIndex+0]
		recentOffs[3] = int32(offset)

		if (offsIndex+1)&4 != 0 {
			offsPos++
		}

		if offset < dstStart-dst {
			return false
		}
		copyfrom := dst + offset
		if matchlen != 15 {
			length := matchlen + 2
			matchCopy(out, dst, copyfrom, length)
			dst += length
		} else {
			matchlen = 14 + int(lenStream[lenPos])
			lenPos++
			if matchlen > dstEnd-dst {
				return false
			}
			matchCopy(out, dst, copyfrom, matchlen)
			dst += matchlen
		}
	}

	if offsPos != offsEnd || lenPos != lenEnd {
		return false
	}

	finalLen := dstEnd - dst
	if finalLen != litEnd-litPos {
		return false
	}
	copy(out[dst:dst+finalLen], lit[litPos:litPos+finalLen])
	return true
}

// processLzRunsType0 ports Kraken_ProcessLzRuns_Type0 (subtract literals).
func processLzRunsType0(lz *krakenLzTable, out []byte, dst, dstEnd, dstStart int) bool {
	cmd := lz.cmdStream
	cmdPos, cmdEnd := 0, lz.cmdStreamSize
	lenStream := lz.lenStream
	lenPos, lenEnd := 0, lz.lenStreamSize
	lit := lz.litStream
	litPos, litEnd := 0, lz.litStreamSize
	offs := lz.offsStream
	offsPos, offsEnd := 0, lz.offsStreamSize

	var recentOffs [7]int32
	recentOffs[3] = -8
	recentOffs[4] = -8
	recentOffs[5] = -8
	lastOffset := -8

	for cmdPos < cmdEnd {
		f := uint32(cmd[cmdPos])
		cmdPos++
		litlen := int(f & 3)
		offsIndex := int(f >> 6)
		matchlen := int((f >> 2) & 0xF)

		nextLongLength := int(lenStream[lenPos])
		if litlen == 3 {
			litlen = nextLongLength
			lenPos++
		}
		recentOffs[6] = offs[offsPos]

		for i := 0; i < litlen; i++ {
			out[dst+i] = lit[litPos+i] + out[dst+i+lastOffset]
		}
		dst += litlen
		litPos += litlen

		offset := int(recentOffs[offsIndex+3])
		recentOffs[offsIndex+3] = recentOffs[offsIndex+2]
		recentOffs[offsIndex+2] = recentOffs[offsIndex+1]
		recentOffs[offsIndex+1] = recentOffs[offsIndex+0]
		recentOffs[3] = int32(offset)
		lastOffset = offset

		if (offsIndex+1)&4 != 0 {
			offsPos++
		}

		if offset < dstStart-dst {
			return false
		}
		copyfrom := dst + offset
		if matchlen != 15 {
			length := matchlen + 2
			matchCopy(out, dst, copyfrom, length)
			dst += length
		} else {
			matchlen = 14 + int(lenStream[lenPos])
			lenPos++
			if matchlen > dstEnd-dst {
				return false
			}
			matchCopy(out, dst, copyfrom, matchlen)
			dst += matchlen
		}
	}

	if offsPos != offsEnd || lenPos != lenEnd {
		return false
	}

	finalLen := dstEnd - dst
	if finalLen != litEnd-litPos {
		return false
	}
	for i := 0; i < finalLen; i++ {
		out[dst+i] = lit[litPos+i] + out[dst+i+lastOffset]
	}
	return true
}

// krakenProcessLzRuns ports Kraken_ProcessLzRuns.
func krakenProcessLzRuns(mode int, out []byte, offset, dstSize int, lz *krakenLzTable) bool {
	dstEnd := offset + dstSize
	start := offset
	if offset == 0 {
		start = offset + 8
	}
	// dst_start = dst - offset = 0 in absolute coordinates.
	if mode == 1 {
		return processLzRunsType1(lz, out, start, dstEnd, 0)
	}
	if mode == 0 {
		return processLzRunsType0(lz, out, start, dstEnd, 0)
	}
	return false
}

// krakenDecodeQuantum ports Kraken_DecodeQuantum. out is the full destination;
// [dstPos, dstEndPos) is the quantum region. block is the compressed quantum.
func krakenDecodeQuantum(out []byte, dstPos, dstEndPos int, block, scratch []byte) int {
	srcPos := 0
	srcEnd := len(block)

	for dstEndPos-dstPos != 0 {
		dstCount := dstEndPos - dstPos
		if dstCount > 0x20000 {
			dstCount = 0x20000
		}
		if srcEnd-srcPos < 4 {
			return -1
		}
		chunkhdr := int(block[srcPos+2]) | int(block[srcPos+1])<<8 | int(block[srcPos])<<16
		if chunkhdr&0x800000 == 0 {
			// Stored as entropy without any match copying.
			chunk, written, srcUsed := krakenDecodeBytes(out[dstPos:], block[srcPos:], dstCount, false, scratch)
			if srcUsed < 0 || written != dstCount {
				return -1
			}
			// Ensure the decoded bytes land in the destination (a no-copy chunk
			// aliases src; a decode-in-place returns the dst region itself).
			if len(chunk) >= written {
				copy(out[dstPos:dstPos+written], chunk[:written])
			}
			srcPos += srcUsed
		} else {
			srcPos += 3
			srcUsed := chunkhdr & 0x7FFFF
			mode := (chunkhdr >> 19) & 0xF
			if srcEnd-srcPos < srcUsed {
				return -1
			}
			if srcUsed < dstCount {
				var lz krakenLzTable
				if !krakenReadLzTable(mode, block[srcPos:srcPos+srcUsed], out, dstPos, dstCount, scratch, &lz) {
					return -1
				}
				if !krakenProcessLzRuns(mode, out, dstPos, dstCount, &lz) {
					return -1
				}
			} else if srcUsed > dstCount || mode != 0 {
				return -1
			} else {
				copy(out[dstPos:dstPos+dstCount], block[srcPos:srcPos+dstCount])
			}
			srcPos += srcUsed
		}
		dstPos += dstCount
	}
	return srcPos
}

// krakenCopyWholeMatch ports Kraken_CopyWholeMatch.
func krakenCopyWholeMatch(out []byte, dstPos int, offset uint32, length int) {
	i := 0
	src := dstPos - int(offset)
	if offset >= 8 {
		for ; i+8 <= length; i += 8 {
			st64(out, dstPos+i, rd64(out, src+i))
		}
	}
	for ; i < length; i++ {
		out[dstPos+i] = out[src+i]
	}
}

// decodeStep ports Kraken_DecodeStep. Returns (srcUsed, dstUsed). srcUsed==0
// signals "not enough input" (a failure to the caller).
func (dec *krakenDecoder) decodeStep(out []byte, offset, dstBytesLeftIn int, src []byte, srcPos, srcBytesLeft int) (int, int, error) {
	srcIn := srcPos
	srcEnd := srcPos + srcBytesLeft

	if offset&0x3FFFF == 0 {
		np, ok := dec.hdr.parse(src, srcPos)
		if !ok {
			return 0, 0, errors.New("oodle: invalid kraken block header")
		}
		srcPos = np
	}

	isKraken := dec.hdr.decoderType == 6 || dec.hdr.decoderType == 10 || dec.hdr.decoderType == 12
	dstBytesLeft := 0x40000
	if !isKraken {
		dstBytesLeft = 0x4000
	}
	if dstBytesLeft > dstBytesLeftIn {
		dstBytesLeft = dstBytesLeftIn
	}

	if dec.hdr.uncompressed {
		if srcEnd-srcPos < dstBytesLeft {
			return 0, 0, nil
		}
		copy(out[offset:offset+dstBytesLeft], src[srcPos:srcPos+dstBytesLeft])
		return (srcPos - srcIn) + dstBytesLeft, dstBytesLeft, nil
	}

	switch dec.hdr.decoderType {
	case 6, 10: // Kraken, Mermaid/Selkie
	default:
		return 0, 0, fmt.Errorf("%w: decoder_type %d", ErrUnsupported, dec.hdr.decoderType)
	}

	var qhdr krakenQuantumHeader
	np, ok := qhdr.parse(src, srcPos, dec.hdr.useChecksums)
	if !ok || np > srcEnd {
		return 0, 0, errors.New("oodle: invalid quantum header")
	}
	srcPos = np

	if srcEnd-srcPos < int(qhdr.compressedSize) {
		return 0, 0, nil
	}
	if int(qhdr.compressedSize) > dstBytesLeft {
		return 0, 0, errors.New("oodle: compressed_size exceeds dst_bytes_left")
	}

	if qhdr.compressedSize == 0 {
		if qhdr.wholeMatchDistance != 0 {
			if qhdr.wholeMatchDistance > uint32(offset) {
				return 0, 0, errors.New("oodle: whole_match_distance exceeds offset")
			}
			krakenCopyWholeMatch(out, offset, qhdr.wholeMatchDistance, dstBytesLeft)
		} else {
			fillByte(out, offset, byte(qhdr.checksum), dstBytesLeft)
		}
		return srcPos - srcIn, dstBytesLeft, nil
	}

	// Checksums are ignored (the reference Kraken_GetCrc is a no-op).

	if int(qhdr.compressedSize) == dstBytesLeft {
		copy(out[offset:offset+dstBytesLeft], src[srcPos:srcPos+dstBytesLeft])
		return (srcPos - srcIn) + dstBytesLeft, dstBytesLeft, nil
	}

	block := src[srcPos : srcPos+int(qhdr.compressedSize)]
	var n int
	switch dec.hdr.decoderType {
	case 6:
		n = krakenDecodeQuantum(out, offset, offset+dstBytesLeft, block, dec.scratch)
	case 10:
		n = mermaidDecodeQuantum(out, offset, offset+dstBytesLeft, block, dec.scratch)
	}
	if n != int(qhdr.compressedSize) {
		return 0, 0, errors.New("oodle: quantum decode size mismatch")
	}
	return (srcPos - srcIn) + n, dstBytesLeft, nil
}

// krakenDecompress ports Kraken_Decompress: the top-level loop over 256K
// blocks. It decodes a full Oodle stream into dst, returning bytes written.
func krakenDecompress(src, dst []byte) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			n = 0
			err = fmt.Errorf("oodle: kraken decode panic: %v", r)
		}
	}()

	// Pad source (over-reads) and destination (word-granular writes) so the
	// C-style over-reads/over-writes never run off the Go allocations.
	srcBuf := make([]byte, len(src)+safePad)
	copy(srcBuf, src)
	out := make([]byte, len(dst)+safePad)

	dec := &krakenDecoder{scratch: make([]byte, scratchSize+safePad)}

	offset := 0
	srcPos := 0
	srcLen := len(src)

	for offset < len(dst) {
		srcUsed, dstUsed, e := dec.decodeStep(out, offset, len(dst)-offset, srcBuf, srcPos, srcLen)
		if e != nil {
			return 0, e
		}
		if srcUsed == 0 {
			return 0, errors.New("oodle: decode step made no progress")
		}
		srcPos += srcUsed
		srcLen -= srcUsed
		offset += dstUsed
	}
	if srcLen != 0 {
		return 0, errors.New("oodle: trailing compressed data after output filled")
	}

	copy(dst, out[:len(dst)])
	return len(dst), nil
}
