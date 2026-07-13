package oodle

import (
	"math/bits"
	"unsafe"
)

// Byte-block level decoders. Ported from ooz kraken.cpp.

var bitmasks = [32]uint32{
	0x1, 0x3, 0x7, 0xf, 0x1f, 0x3f, 0x7f,
	0xff, 0x1ff, 0x3ff, 0x7ff, 0xfff, 0x1fff, 0x3fff,
	0x7fff, 0xffff, 0x1ffff, 0x3ffff, 0x7ffff, 0xfffff, 0x1fffff,
	0x3fffff, 0x7fffff, 0xffffff, 0x1ffffff, 0x3ffffff, 0x7ffffff, 0xfffffff,
	0x1fffffff, 0x3fffffff, 0x7fffffff, 0xffffffff,
}

// sameBase reports whether two slices share the same backing array origin.
// Mirrors the C "dst == scratch" pointer identity check.
func sameBase(a, b []byte) bool {
	pa := unsafe.SliceData(a)
	pb := unsafe.SliceData(b)
	return pa != nil && pa == pb
}

// krakenGetBlockSize ports Kraken_GetBlockSize. Returns destSize and a
// non-negative marker on success, -1 on failure (only the sign is used by
// callers).
func krakenGetBlockSize(src []byte, destCapacity int) (destSize int, ok int) {
	srcLen := len(src)
	if srcLen < 2 {
		return 0, -1
	}
	chunkType := int(src[0]>>4) & 0x7
	if chunkType == 0 {
		var srcSize, srcPos int
		if src[0] >= 0x80 {
			srcSize = int((uint32(src[0])<<8 | uint32(src[1])) & 0xFFF)
			srcPos = 2
		} else {
			if srcLen < 3 {
				return 0, -1
			}
			srcSize = int(uint32(src[0])<<16 | uint32(src[1])<<8 | uint32(src[2]))
			if srcSize&^0x3ffff != 0 {
				return 0, -1
			}
			srcPos = 3
		}
		if srcSize > destCapacity || srcLen-srcPos < srcSize {
			return 0, -1
		}
		return srcSize, srcPos + srcSize
	}
	if chunkType >= 6 {
		return 0, -1
	}
	var srcSize, dstSize, srcPos int
	if src[0] >= 0x80 {
		if srcLen < 3 {
			return 0, -1
		}
		b := uint32(src[0])<<16 | uint32(src[1])<<8 | uint32(src[2])
		srcSize = int(b & 0x3ff)
		dstSize = srcSize + int((b>>10)&0x3ff) + 1
		srcPos = 3
	} else {
		if srcLen < 5 {
			return 0, -1
		}
		b := uint32(src[1])<<24 | uint32(src[2])<<16 | uint32(src[3])<<8 | uint32(src[4])
		srcSize = int(b & 0x3ffff)
		dstSize = int(((b>>18)|(uint32(src[0])<<14))&0x3FFFF) + 1
		if srcSize >= dstSize {
			return 0, -1
		}
		srcPos = 5
	}
	if srcLen-srcPos < srcSize || dstSize > destCapacity {
		return 0, -1
	}
	return dstSize, srcSize
}

// krakenDecodeBytes ports Kraken_DecodeBytes.
//
// Returns out (slice holding decodedSize bytes; may alias src for no-copy),
// decodedSize, and srcUsed. srcUsed < 0 signals an error.
func krakenDecodeBytes(output, src []byte, outputSize int, forceMemmove bool, scratch []byte) (out []byte, decodedSize int, srcUsed int) {
	srcLen := len(src)
	if srcLen < 2 {
		return nil, 0, -1
	}
	chunkType := int(src[0]>>4) & 0x7
	srcPos := 0
	var srcSize, dstSize int
	if chunkType == 0 {
		if src[0] >= 0x80 {
			srcSize = int((uint32(src[0])<<8 | uint32(src[1])) & 0xFFF)
			srcPos += 2
		} else {
			if srcLen < 3 {
				return nil, 0, -1
			}
			srcSize = int(uint32(src[0])<<16 | uint32(src[1])<<8 | uint32(src[2]))
			if srcSize&^0x3ffff != 0 {
				return nil, 0, -1
			}
			srcPos += 3
		}
		if srcSize > outputSize || srcLen-srcPos < srcSize {
			return nil, 0, -1
		}
		decodedSize = srcSize
		if forceMemmove {
			copy(output[:srcSize], src[srcPos:srcPos+srcSize])
			out = output
		} else {
			out = src[srcPos:]
		}
		return out, decodedSize, srcPos + srcSize
	}

	if src[0] >= 0x80 {
		if srcLen < 3 {
			return nil, 0, -1
		}
		b := uint32(src[0])<<16 | uint32(src[1])<<8 | uint32(src[2])
		srcSize = int(b & 0x3ff)
		dstSize = srcSize + int((b>>10)&0x3ff) + 1
		srcPos += 3
	} else {
		if srcLen < 5 {
			return nil, 0, -1
		}
		b := uint32(src[1])<<24 | uint32(src[2])<<16 | uint32(src[3])<<8 | uint32(src[4])
		srcSize = int(b & 0x3ffff)
		dstSize = int(((b>>18)|(uint32(src[0])<<14))&0x3FFFF) + 1
		if srcSize >= dstSize {
			return nil, 0, -1
		}
		srcPos += 5
	}
	if srcLen-srcPos < srcSize || dstSize > outputSize {
		return nil, 0, -1
	}

	dst := output
	if sameBase(dst, scratch) {
		if len(scratch) < dstSize {
			return nil, 0, -1
		}
		scratch = scratch[dstSize:]
	}

	block := src[srcPos : srcPos+srcSize]
	var srcUsedInner int
	switch chunkType {
	case 2, 4:
		srcUsedInner = decodeBytesType12(block, srcSize, dst, dstSize, chunkType>>1)
	case 5:
		srcUsedInner = krakDecodeRecursive(block, srcSize, dst, dstSize, scratch)
	case 3:
		srcUsedInner = krakDecodeRLE(block, srcSize, dst, dstSize, scratch)
	case 1:
		srcUsedInner = krakDecodeTans(block, srcSize, dst, dstSize, scratch)
	default:
		srcUsedInner = -1
	}
	if srcUsedInner != srcSize {
		return nil, 0, -1
	}
	decodedSize = dstSize
	out = dst
	return out, decodedSize, srcPos + srcSize
}

// krakDecodeRecursive ports Krak_DecodeRecursive.
func krakDecodeRecursive(src []byte, srcSize int, output []byte, outputSize int, scratch []byte) int {
	if srcSize < 6 {
		return -1
	}
	n := int(src[0]) & 0x7f
	if n < 2 {
		return -1
	}

	if src[0]&0x80 == 0 {
		srcPos := 1
		outPos := 0
		for {
			block := src[srcPos:]
			outSlice, decodedSize, dec := krakenDecodeBytes(output[outPos:], block, outputSize-outPos, true, scratch)
			if dec < 0 {
				return -1
			}
			// force_memmove=true so data is written into output[outPos:]
			_ = outSlice
			outPos += decodedSize
			srcPos += dec
			n--
			if n == 0 {
				break
			}
		}
		if outPos != outputSize {
			return -1
		}
		return srcPos
	}

	var arrayData [1][]byte
	var arrayLens [1]int
	decodedSize, dec := krakenDecodeMultiArray(src, output, outputSize, arrayData[:], arrayLens[:], 1, true, scratch)
	if dec < 0 {
		return -1
	}
	if decodedSize != outputSize {
		return -1
	}
	return dec
}

// krakDecodeRLE ports Krak_DecodeRLE.
func krakDecodeRLE(src []byte, srcSize int, dst []byte, dstSize int, scratch []byte) int {
	if srcSize <= 1 {
		if srcSize != 1 {
			return -1
		}
		fillByte(dst, 0, src[0], dstSize)
		return 1
	}
	dstPos := 0
	dstEnd := dstSize

	// Command buffer: either directly src[1:srcSize], or a decoded scratch buffer.
	cmdBuf := src
	cmdPtr := 1
	cmdPtrEnd := srcSize

	if src[0] != 0 {
		out, decSize, n := krakenDecodeBytes(scratch, src, len(scratch), true, scratch)
		if n <= 0 {
			return -1
		}
		cmdLen := srcSize - n + decSize
		if cmdLen > len(scratch) {
			return -1
		}
		// out is scratch[:...]; append the tail src[n:] after the decoded prefix.
		copy(out[decSize:decSize+(srcSize-n)], src[n:srcSize])
		cmdBuf = out
		cmdPtr = 0
		cmdPtrEnd = cmdLen
	}

	rleByte := byte(0)

	for cmdPtr < cmdPtrEnd {
		cmd := uint32(cmdBuf[cmdPtrEnd-1])
		if cmd-1 >= 0x2f {
			cmdPtrEnd--
			bytesToCopy := int((^cmd) & 0xF) // (-1 - cmd) & 0xF
			bytesToRle := int(cmd >> 4)
			if dstEnd-dstPos < bytesToCopy+bytesToRle || cmdPtrEnd-cmdPtr < bytesToCopy {
				return -1
			}
			copy(dst[dstPos:dstPos+bytesToCopy], cmdBuf[cmdPtr:cmdPtr+bytesToCopy])
			cmdPtr += bytesToCopy
			dstPos += bytesToCopy
			fillByte(dst, dstPos, rleByte, bytesToRle)
			dstPos += bytesToRle
		} else if cmd >= 0x10 {
			data := rd16(cmdBuf, cmdPtrEnd-2) - 4096
			cmdPtrEnd -= 2
			bytesToCopy := int(data & 0x3F)
			bytesToRle := int(data >> 6)
			if dstEnd-dstPos < bytesToCopy+bytesToRle || cmdPtrEnd-cmdPtr < bytesToCopy {
				return -1
			}
			copy(dst[dstPos:dstPos+bytesToCopy], cmdBuf[cmdPtr:cmdPtr+bytesToCopy])
			cmdPtr += bytesToCopy
			dstPos += bytesToCopy
			fillByte(dst, dstPos, rleByte, bytesToRle)
			dstPos += bytesToRle
		} else if cmd == 1 {
			rleByte = cmdBuf[cmdPtr]
			cmdPtr++
			cmdPtrEnd--
		} else if cmd >= 9 {
			bytesToRle := int((rd16(cmdBuf, cmdPtrEnd-2) - 0x8ff) * 128)
			cmdPtrEnd -= 2
			if dstEnd-dstPos < bytesToRle {
				return -1
			}
			fillByte(dst, dstPos, rleByte, bytesToRle)
			dstPos += bytesToRle
		} else {
			bytesToCopy := int((rd16(cmdBuf, cmdPtrEnd-2) - 511) * 64)
			cmdPtrEnd -= 2
			if cmdPtrEnd-cmdPtr < bytesToCopy || dstEnd-dstPos < bytesToCopy {
				return -1
			}
			copy(dst[dstPos:dstPos+bytesToCopy], cmdBuf[cmdPtr:cmdPtr+bytesToCopy])
			dstPos += bytesToCopy
			cmdPtr += bytesToCopy
		}
	}
	if cmdPtrEnd != cmdPtr {
		return -1
	}
	if dstPos != dstEnd {
		return -1
	}
	return srcSize
}

// krakenDecodeMultiArray ports Kraken_DecodeMultiArray.
//
// dst is the destination buffer; arrays are written contiguously starting at
// index 0. Returns totalSize and srcUsed (srcUsed < 0 on error).
func krakenDecodeMultiArray(src []byte, dst []byte, dstCap int, arrayData [][]byte, arrayLens []int, arrayCount int, forceMemmove bool, scratch []byte) (totalSizeOut int, srcUsed int) {
	srcLen := len(src)
	srcPos := 0
	if srcLen < 4 {
		return 0, -1
	}

	numArraysInFile := int(src[srcPos])
	srcPos++
	if numArraysInFile&0x80 == 0 {
		return 0, -1
	}
	numArraysInFile &= 0x3f

	dstPos := 0
	dstEnd := dstCap
	if sameBase(dst, scratch) {
		// scratch += (scratch_end - scratch - 0xc000) >> 1; dst_end = scratch
		off := (len(scratch) - 0xc000) >> 1
		scratch = scratch[off:]
		dstEnd = off
	}

	totalSize := 0

	if numArraysInFile == 0 {
		for i := 0; i < arrayCount; i++ {
			block := src[srcPos:]
			chunk, decodedSize, dec := krakenDecodeBytes(dst[dstPos:], block, dstEnd-dstPos, forceMemmove, scratch)
			if dec < 0 {
				return 0, -1
			}
			arrayLens[i] = decodedSize
			arrayData[i] = chunk
			dstPos += decodedSize
			srcPos += dec
			totalSize += decodedSize
		}
		totalSizeOut = totalSize
		return totalSizeOut, srcPos
	}

	entropyArrayData := make([][]byte, 63)
	entropyArraySize := make([]uint32, 63)

	scratchCur := scratch
	scratchUsed := 0 // bytes consumed from scratchCur front for entropy arrays

	for i := 0; i < numArraysInFile; i++ {
		block := src[srcPos:]
		chunk, decodedSize, dec := krakenDecodeBytes(scratchCur, block, len(scratchCur), forceMemmove, scratchCur)
		if dec < 0 {
			return 0, -1
		}
		entropyArrayData[i] = chunk
		entropyArraySize[i] = uint32(decodedSize)
		scratchCur = scratchCur[decodedSize:]
		scratchUsed += decodedSize
		totalSize += decodedSize
		srcPos += dec
	}
	totalSizeOut = totalSize

	if srcLen-srcPos < 3 {
		return 0, -1
	}

	Q := int(rd16(src, srcPos))
	srcPos += 2

	outSize, gb := krakenGetBlockSize(src[srcPos:], totalSize)
	if gb < 0 {
		return 0, -1
	}
	numIndexes := outSize

	numLens := numIndexes - arrayCount
	if numLens < 1 {
		return 0, -1
	}

	if len(scratchCur) < numIndexes {
		return 0, -1
	}
	intervalLenlog2 := scratchCur[:numIndexes]
	scratchCur = scratchCur[numIndexes:]

	if len(scratchCur) < numIndexes {
		return 0, -1
	}
	intervalIndexes := scratchCur[:numIndexes]
	scratchCur = scratchCur[numIndexes:]

	if Q&0x8000 != 0 {
		chunk, sizeOut, n := krakenDecodeBytes(intervalIndexes, src[srcPos:], numIndexes, true, scratchCur)
		if n < 0 || sizeOut != numIndexes {
			return 0, -1
		}
		intervalIndexes = chunk
		srcPos += n

		for i := 0; i < numIndexes; i++ {
			t := int(intervalIndexes[i])
			intervalLenlog2[i] = byte(t >> 4)
			intervalIndexes[i] = byte(t & 0xF)
		}
		numLens = numIndexes
	} else {
		lenlog2Chunksize := numIndexes - arrayCount

		chunk, sizeOut, n := krakenDecodeBytes(intervalIndexes, src[srcPos:], numIndexes, false, scratchCur)
		if n < 0 || sizeOut != numIndexes {
			return 0, -1
		}
		intervalIndexes = chunk
		srcPos += n

		chunk2, sizeOut2, n2 := krakenDecodeBytes(intervalLenlog2, src[srcPos:], lenlog2Chunksize, false, scratchCur)
		if n2 < 0 || sizeOut2 != lenlog2Chunksize {
			return 0, -1
		}
		intervalLenlog2 = chunk2
		srcPos += n2

		for i := 0; i < lenlog2Chunksize; i++ {
			if intervalLenlog2[i] > 16 {
				return 0, -1
			}
		}
	}

	decodedIntervals := make([]uint32, numLens)

	varbitsComplen := Q & 0x3FFF
	if srcLen-srcPos < varbitsComplen {
		return 0, -1
	}

	f := srcPos
	bitsF := uint32(0)
	bitposF := 24

	srcEndActual := srcPos + varbitsComplen

	bpos := srcEndActual
	bitsB := uint32(0)
	bitposB := 24

	i := 0
	for ; i+2 <= numLens; i += 2 {
		bitsF |= bswap32(src, f) >> uint(24-bitposF)
		f += (bitposF + 7) >> 3

		bitsB |= rd32(src, bpos-4) >> uint(24-bitposB)
		bpos -= (bitposB + 7) >> 3

		numbitsF := int(intervalLenlog2[i+0])
		numbitsB := int(intervalLenlog2[i+1])

		bitsF = bits.RotateLeft32(bitsF|1, numbitsF)
		bitposF += numbitsF - 8*((bitposF+7)>>3)

		bitsB = bits.RotateLeft32(bitsB|1, numbitsB)
		bitposB += numbitsB - 8*((bitposB+7)>>3)

		valueF := bitsF & bitmasks[numbitsF]
		bitsF &^= bitmasks[numbitsF]

		valueB := bitsB & bitmasks[numbitsB]
		bitsB &^= bitmasks[numbitsB]

		decodedIntervals[i+0] = valueF
		decodedIntervals[i+1] = valueB
	}

	if i < numLens {
		bitsF |= bswap32(src, f) >> uint(24-bitposF)
		numbitsF := int(intervalLenlog2[i])
		bitsF = bits.RotateLeft32(bitsF|1, numbitsF)
		valueF := bitsF & bitmasks[numbitsF]
		decodedIntervals[i+0] = valueF
	}

	if intervalIndexes[numIndexes-1] != 0 {
		return 0, -1
	}

	indi := 0
	leni := 0
	incrementLeni := 0
	if Q&0x8000 != 0 {
		incrementLeni = 1
	}

	for arri := 0; arri < arrayCount; arri++ {
		arrayStart := dstPos
		arrayData[arri] = dst[dstPos:]
		if indi >= numIndexes {
			return 0, -1
		}

		for {
			source := int(intervalIndexes[indi])
			indi++
			if source == 0 {
				break
			}
			if source > numArraysInFile {
				return 0, -1
			}
			if leni >= numLens {
				return 0, -1
			}
			curLen := int(decodedIntervals[leni])
			leni++
			bytesLeft := int(entropyArraySize[source-1])
			if curLen > bytesLeft || curLen > dstEnd-dstPos {
				return 0, -1
			}
			blksrc := entropyArrayData[source-1]
			entropyArraySize[source-1] -= uint32(curLen)
			entropyArrayData[source-1] = blksrc[curLen:]
			copy(dst[dstPos:dstPos+curLen], blksrc[:curLen])
			dstPos += curLen
		}
		leni += incrementLeni
		arrayLens[arri] = dstPos - arrayStart
	}

	if indi != numIndexes || leni != numLens {
		return 0, -1
	}

	for i := 0; i < numArraysInFile; i++ {
		if entropyArraySize[i] != 0 {
			return 0, -1
		}
	}
	return totalSizeOut, srcEndActual
}
