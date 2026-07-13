package oodle

// Mermaid/Selkie decompressor (decoder_type 10). Ported from ooz kraken.cpp.
// Palworld .sav PlM streams use Mermaid, so this is the primary path.

// mermaidLzTable mirrors struct MermaidLzTable, adapted to Go slices+indices.
type mermaidLzTable struct {
	block []byte // the compressed quantum block (length_stream lives here)

	cmd    []byte
	cmdPos int
	cmdEnd int

	cmdStream2Offs    int
	cmdStream2OffsEnd int

	lengthPos int
	lengthEnd int

	lit    []byte
	litPos int
	litEnd int

	off16    []uint16
	off16Pos int
	off16End int

	off32    []uint32
	off32Pos int
	off32End int

	off32_1    []uint32
	off32Size1 int
	off32_2    []uint32
	off32Size2 int
}

// mermaidDecodeFarOffsets ports Mermaid_DecodeFarOffsets. Reads outputSize far
// offsets from block[srcPos:srcEnd] into output. Returns bytes consumed or -1.
func mermaidDecodeFarOffsets(block []byte, srcPos, srcEnd int, output []uint32, outputSize int, offset int64) int {
	cur := srcPos
	if offset < (0xC00000 - 1) {
		for i := 0; i < outputSize; i++ {
			if srcEnd-cur < 3 {
				return -1
			}
			off := uint32(block[cur]) | uint32(block[cur+1])<<8 | uint32(block[cur+2])<<16
			cur += 3
			output[i] = off
			if int64(off) > offset {
				return -1
			}
		}
		return cur - srcPos
	}
	for i := 0; i < outputSize; i++ {
		if srcEnd-cur < 3 {
			return -1
		}
		off := uint32(block[cur]) | uint32(block[cur+1])<<8 | uint32(block[cur+2])<<16
		cur += 3
		if off >= 0xc00000 {
			if cur == srcEnd {
				return -1
			}
			off += uint32(block[cur]) << 22
			cur++
		}
		output[i] = off
		if int64(off) > offset {
			return -1
		}
	}
	return cur - srcPos
}

// mermaidReadLzTable ports Mermaid_ReadLzTable. out is the full destination;
// offset is the current absolute write position.
func mermaidReadLzTable(mode int, block, out []byte, offset, dstSize int, scratch []byte, lz *mermaidLzTable) bool {
	if mode > 1 {
		return false
	}
	srcEnd := len(block)
	srcPos := 0
	if srcEnd < 10 {
		return false
	}

	lz.block = block
	lz.lengthEnd = srcEnd

	if offset == 0 {
		copy(out[offset:offset+8], block[0:8])
		srcPos += 8
	}

	scr := scratch

	// Decode lit stream.
	outLit, dcLit, n := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), dstSize), false, scr)
	if n < 0 {
		return false
	}
	srcPos += n
	lz.lit = outLit
	lz.litPos = 0
	lz.litEnd = dcLit
	scr = scr[dcLit:]

	// Decode flag/cmd stream.
	outCmd, dcCmd, n := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), dstSize), false, scr)
	if n < 0 {
		return false
	}
	srcPos += n
	lz.cmd = outCmd
	scr = scr[dcCmd:]

	lz.cmdStream2OffsEnd = dcCmd
	if dstSize <= 0x10000 {
		lz.cmdStream2Offs = dcCmd
	} else {
		if srcEnd-srcPos < 2 {
			return false
		}
		lz.cmdStream2Offs = int(rd16(block, srcPos))
		srcPos += 2
		if lz.cmdStream2Offs > lz.cmdStream2OffsEnd {
			return false
		}
	}

	if srcEnd-srcPos < 2 {
		return false
	}
	off16Count := int(rd16(block, srcPos))
	if off16Count == 0xffff {
		// off16 is entropy coded (hi then lo bytes).
		srcPos += 2
		off16hi, hiCount, nn := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), dstSize>>1), false, scr)
		if nn < 0 {
			return false
		}
		srcPos += nn
		scr = scr[hiCount:]

		off16lo, loCount, nn2 := krakenDecodeBytes(scr, block[srcPos:], min(len(scr), dstSize>>1), false, scr)
		if nn2 < 0 {
			return false
		}
		srcPos += nn2
		scr = scr[loCount:]

		if loCount != hiCount {
			return false
		}
		o := make([]uint16, loCount+4)
		for i := 0; i < loCount; i++ {
			o[i] = uint16(off16lo[i]) + uint16(off16hi[i])*256
		}
		lz.off16 = o
		lz.off16Pos = 0
		lz.off16End = loCount
	} else {
		o := make([]uint16, off16Count+4)
		for i := 0; i < off16Count; i++ {
			o[i] = uint16(rd16(block, srcPos+2+i*2))
		}
		srcPos += 2 + off16Count*2
		lz.off16 = o
		lz.off16Pos = 0
		lz.off16End = off16Count
	}

	if srcEnd-srcPos < 3 {
		return false
	}
	tmp := uint32(block[srcPos]) | uint32(block[srcPos+1])<<8 | uint32(block[srcPos+2])<<16
	srcPos += 3

	if tmp != 0 {
		off32Size1 := int(tmp >> 12)
		off32Size2 := int(tmp & 0xFFF)
		if off32Size1 == 4095 {
			if srcEnd-srcPos < 2 {
				return false
			}
			off32Size1 = int(rd16(block, srcPos))
			srcPos += 2
		}
		if off32Size2 == 4095 {
			if srcEnd-srcPos < 2 {
				return false
			}
			off32Size2 = int(rd16(block, srcPos))
			srcPos += 2
		}
		lz.off32Size1 = off32Size1
		lz.off32Size2 = off32Size2
		lz.off32_1 = make([]uint32, off32Size1+8)
		lz.off32_2 = make([]uint32, off32Size2+8)

		n = mermaidDecodeFarOffsets(block, srcPos, srcEnd, lz.off32_1, off32Size1, int64(offset))
		if n < 0 {
			return false
		}
		srcPos += n

		n = mermaidDecodeFarOffsets(block, srcPos, srcEnd, lz.off32_2, off32Size2, int64(offset)+0x10000)
		if n < 0 {
			return false
		}
		srcPos += n
	} else {
		lz.off32Size1 = 0
		lz.off32Size2 = 0
		lz.off32_1 = make([]uint32, 8)
		lz.off32_2 = make([]uint32, 8)
	}

	lz.lengthPos = srcPos
	return true
}

// mermaidReadLen reads a Mermaid length code from block at *lengthPos.
// Returns the base length and advances *lengthPos, or ok=false on overrun.
func mermaidReadLen(block []byte, lengthPos *int, lengthEnd int) (int, bool) {
	if lengthEnd-*lengthPos == 0 {
		return 0, false
	}
	length := int(block[*lengthPos])
	if length > 251 {
		if lengthEnd-*lengthPos < 3 {
			return 0, false
		}
		length += int(rd16(block, *lengthPos+1)) * 4
		*lengthPos += 2
	}
	*lengthPos++
	return length, true
}

// mermaidMode ports Mermaid_Mode0 (addLit=true) / Mermaid_Mode1 (addLit=false).
// Returns the updated length_stream position, or -1 on failure.
func mermaidMode(out []byte, dstBegin, dstSize, dstStart int, lz *mermaidLzTable, savedDist *int, startoff int, addLit bool) int {
	dstEndLocal := dstBegin + dstSize
	block := lz.block
	lengthEnd := lz.lengthEnd
	lengthPos := lz.lengthPos
	cmd := lz.cmd
	cmdPos := lz.cmdPos
	cmdEnd := lz.cmdEnd
	lit := lz.lit
	litPos := lz.litPos
	litEnd := lz.litEnd
	off16 := lz.off16
	off16Pos := lz.off16Pos
	off16End := lz.off16End
	off32 := lz.off32
	off32Pos := lz.off32Pos
	off32End := lz.off32End
	recentOffs := *savedDist

	dst := dstBegin + startoff

	for cmdPos < cmdEnd {
		c := int(cmd[cmdPos])
		cmdPos++
		if c >= 24 {
			newDist := 0
			if off16Pos < len(off16) {
				newDist = int(off16[off16Pos])
			}
			useNew := (c >> 7) == 0
			litlen := c & 7
			if addLit {
				for i := 0; i < litlen; i++ {
					out[dst+i] = lit[litPos+i] + out[dst+i+recentOffs]
				}
			} else {
				copy(out[dst:dst+litlen], lit[litPos:litPos+litlen])
			}
			dst += litlen
			litPos += litlen
			if useNew {
				recentOffs = -newDist
				off16Pos++
			}
			matchPos := dst + recentOffs
			matchlen := (c >> 3) & 0xF
			matchCopy(out, dst, matchPos, matchlen)
			dst += matchlen
		} else if c > 2 {
			length := c + 5
			if off32Pos >= off32End {
				return -1
			}
			matchPos := dstBegin - int(off32[off32Pos])
			off32Pos++
			recentOffs = matchPos - dst
			if dstEndLocal-dst < length {
				return -1
			}
			matchCopy(out, dst, matchPos, length)
			dst += length
		} else if c == 0 {
			length, ok := mermaidReadLen(block, &lengthPos, lengthEnd)
			if !ok {
				return -1
			}
			length += 64
			if dstEndLocal-dst < length || litEnd-litPos < length {
				return -1
			}
			if addLit {
				for i := 0; i < length; i++ {
					out[dst+i] = lit[litPos+i] + out[dst+i+recentOffs]
				}
			} else {
				copy(out[dst:dst+length], lit[litPos:litPos+length])
			}
			dst += length
			litPos += length
		} else if c == 1 {
			length, ok := mermaidReadLen(block, &lengthPos, lengthEnd)
			if !ok {
				return -1
			}
			length += 91
			if off16Pos >= off16End {
				return -1
			}
			matchPos := dst - int(off16[off16Pos])
			off16Pos++
			recentOffs = matchPos - dst
			matchCopy(out, dst, matchPos, length)
			dst += length
		} else { // c == 2
			length, ok := mermaidReadLen(block, &lengthPos, lengthEnd)
			if !ok {
				return -1
			}
			length += 29
			if off32Pos >= off32End {
				return -1
			}
			matchPos := dstBegin - int(off32[off32Pos])
			off32Pos++
			recentOffs = matchPos - dst
			matchCopy(out, dst, matchPos, length)
			dst += length
		}
	}

	// Trailing literals.
	length := dstEndLocal - dst
	if addLit {
		for i := 0; i < length; i++ {
			out[dst+i] = lit[litPos+i] + out[dst+i+recentOffs]
		}
	} else {
		copy(out[dst:dst+length], lit[litPos:litPos+length])
	}
	dst += length
	litPos += length

	*savedDist = recentOffs
	lz.lengthPos = lengthPos
	lz.off16Pos = off16Pos
	lz.litPos = litPos
	return lengthPos
}

// mermaidProcessLzRuns ports Mermaid_ProcessLzRuns.
func mermaidProcessLzRuns(mode int, out []byte, dstPos, dstSize, offset int, lz *mermaidLzTable) bool {
	savedDist := -8
	srcCur := -1
	dst := dstPos
	remaining := dstSize
	addLit := mode == 0

	for iteration := 0; iteration != 2; iteration++ {
		dstSizeCur := remaining
		if dstSizeCur > 0x10000 {
			dstSizeCur = 0x10000
		}

		if iteration == 0 {
			lz.off32 = lz.off32_1
			lz.off32Pos = 0
			lz.off32End = lz.off32Size1
			lz.cmdPos = 0
			lz.cmdEnd = lz.cmdStream2Offs
		} else {
			lz.off32 = lz.off32_2
			lz.off32Pos = 0
			lz.off32End = lz.off32Size2
			lz.cmdPos = lz.cmdStream2Offs
			lz.cmdEnd = lz.cmdStream2OffsEnd
		}

		startoff := 0
		if offset == 0 && iteration == 0 {
			startoff = 8
		}

		srcCur = mermaidMode(out, dst, dstSizeCur, 0, lz, &savedDist, startoff, addLit)
		if srcCur < 0 {
			return false
		}

		dst += dstSizeCur
		remaining -= dstSizeCur
		if remaining == 0 {
			break
		}
	}

	if srcCur != lz.lengthEnd {
		return false
	}
	return true
}

// mermaidDecodeQuantum ports Mermaid_DecodeQuantum.
func mermaidDecodeQuantum(out []byte, dstPos, dstEndPos int, block, scratch []byte) int {
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
			chunk, written, srcUsed := krakenDecodeBytes(out[dstPos:], block[srcPos:], dstCount, false, scratch)
			if srcUsed < 0 || written != dstCount {
				return -1
			}
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
				blk := block[srcPos : srcPos+srcUsed]
				var lz mermaidLzTable
				if !mermaidReadLzTable(mode, blk, out, dstPos, dstCount, scratch, &lz) {
					return -1
				}
				if !mermaidProcessLzRuns(mode, out, dstPos, dstCount, dstPos, &lz) {
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
