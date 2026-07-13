package gvas

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// DecodeFunc is a custom property decoder. It mirrors the reader-side callable
// registered in PALWORLD_CUSTOM_PROPERTIES: given the archive reader positioned
// at the start of the property body, the property type name, its declared size
// and its path, it returns the fully decoded Property.
type DecodeFunc func(r *Reader, typeName string, size int, path string) (Property, error)

// Reader is a read-only cursor over a GVAS blob. It carries the Palworld type
// hints and custom decoders so nested reads resolve struct types and dispatch
// custom decoders exactly like the reference FArchiveReader.
type Reader struct {
	data   []byte
	pos    int
	hints  map[string]string
	custom map[string]DecodeFunc
}

// NewReader builds a Reader over data. hints and custom may be nil.
func NewReader(data []byte, hints map[string]string, custom map[string]DecodeFunc) *Reader {
	return &Reader{data: data, hints: hints, custom: custom}
}

// InternalCopy returns a sub-reader over buf that shares this reader's hints and
// custom decoders. Mirrors FArchiveReader.internal_copy.
func (r *Reader) InternalCopy(buf []byte) *Reader {
	return &Reader{data: buf, hints: r.hints, custom: r.custom}
}

// parseError is panicked on any read failure and recovered at public API
// boundaries, keeping the port close to the exception-based reference.
type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

func (r *Reader) fail(format string, args ...any) {
	panic(&parseError{fmt.Sprintf(format, args...)})
}

// catch converts a parseError panic into *errp; any other panic is re-raised.
func catch(errp *error) {
	if rec := recover(); rec != nil {
		if pe, ok := rec.(*parseError); ok {
			*errp = pe
			return
		}
		panic(rec)
	}
}

// Attempt runs fn and returns any parseError it panics with (nil on success).
// Non-parse panics propagate. Used by rawdata decoders for trial parsing.
func Attempt(fn func()) (err error) {
	defer catch(&err)
	fn()
	return nil
}

// --- cursor management ---

// Pos returns the current byte offset.
func (r *Reader) Pos() int { return r.pos }

// SeekTo sets the current byte offset.
func (r *Reader) SeekTo(pos int) { r.pos = pos }

// Size returns the total length of the underlying buffer.
func (r *Reader) Size() int { return len(r.data) }

// Eof reports whether the cursor is at or past the end of the buffer.
func (r *Reader) Eof() bool { return r.pos >= len(r.data) }

// Skip advances the cursor by n bytes.
func (r *Reader) Skip(n int) {
	if r.pos+n > len(r.data) || n < 0 {
		r.fail("gvas: skip %d out of bounds at %d/%d", n, r.pos, len(r.data))
	}
	r.pos += n
}

func (r *Reader) take(n int) []byte {
	if n < 0 || r.pos+n > len(r.data) {
		r.fail("gvas: read %d out of bounds at %d/%d", n, r.pos, len(r.data))
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

// Read returns the next n bytes (a copy-free subslice).
func (r *Reader) Read(n int) []byte { return r.take(n) }

// ReadToEnd returns the remaining bytes.
func (r *Reader) ReadToEnd() []byte { return r.take(len(r.data) - r.pos) }

// ByteList returns the next size bytes as a fresh copy.
func (r *Reader) ByteList(size int) []byte {
	b := r.take(size)
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// --- primitives ---

func (r *Reader) Byte() uint8 { return r.take(1)[0] }
func (r *Reader) Bool() bool  { return r.Byte() > 0 }
func (r *Reader) I16() int16  { return int16(binary.LittleEndian.Uint16(r.take(2))) }
func (r *Reader) U16() uint16 { return binary.LittleEndian.Uint16(r.take(2)) }
func (r *Reader) I32() int32  { return int32(binary.LittleEndian.Uint32(r.take(4))) }
func (r *Reader) U32() uint32 { return binary.LittleEndian.Uint32(r.take(4)) }
func (r *Reader) I64() int64  { return int64(binary.LittleEndian.Uint64(r.take(8))) }
func (r *Reader) U64() uint64 { return binary.LittleEndian.Uint64(r.take(8)) }
func (r *Reader) F32() float64 {
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(r.take(4))))
}
func (r *Reader) F64() float64 { return math.Float64frombits(binary.LittleEndian.Uint64(r.take(8))) }

// FString reads a length-prefixed UE string. size<0 -> UTF-16LE, size>0 ->
// ASCII, size==0 -> "". The trailing null terminator is stripped. Mirrors
// FArchiveReader.fstring.
func (r *Reader) FString() string {
	size := int(r.I32())
	if size == 0 {
		return ""
	}
	if size < 0 {
		size = -size
		raw := r.take(size * 2)
		// drop trailing \x00\x00
		raw = raw[:len(raw)-2]
		u := make([]uint16, len(raw)/2)
		for i := range u {
			u[i] = binary.LittleEndian.Uint16(raw[i*2:])
		}
		return string(utf16.Decode(u))
	}
	raw := r.take(size)
	// drop trailing \x00
	return string(raw[:len(raw)-1])
}

// GUID reads 16 bytes and formats them as the reference UUID string.
func (r *Reader) GUID() string {
	b := r.take(16)
	return formatGUID(b)
}

func formatGUID(b []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%04x%08x",
		uint32(b[3])<<24|uint32(b[2])<<16|uint32(b[1])<<8|uint32(b[0]),
		uint32(b[7])<<8|uint32(b[6]),
		uint32(b[5])<<8|uint32(b[4]),
		uint32(b[11])<<8|uint32(b[10]),
		uint32(b[9])<<8|uint32(b[8]),
		uint32(b[15])<<24|uint32(b[14])<<16|uint32(b[13])<<8|uint32(b[12]))
}

// OptionalGUID reads a bool flag then, if set, a GUID.
func (r *Reader) OptionalGUID() *string {
	if r.Byte() != 0 {
		g := r.GUID()
		return &g
	}
	return nil
}

// TArray reads a u32 count then count elements via elem.
func (r *Reader) TArray(elem func(*Reader) any) []any {
	count := r.U32()
	out := make([]any, 0, count)
	for i := uint32(0); i < count; i++ {
		out = append(out, elem(r))
	}
	return out
}

// InstanceID mirrors instance_id_reader: {guid, instance_id}.
func (r *Reader) InstanceID() map[string]string {
	return map[string]string{"guid": r.GUID(), "instance_id": r.GUID()}
}

// getTypeOr returns the hint for path or the default.
func (r *Reader) getTypeOr(path, def string) string {
	if r.hints != nil {
		if t, ok := r.hints[path]; ok {
			return t
		}
	}
	return def
}
