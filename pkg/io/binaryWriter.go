package io

import (
	"bytes"
	"encoding/binary"
	"io"
	"reflect"
)

// BinWriter is a convenient wrapper around an io.Writer and err object.
// Used to simplify error handling when writing into an io.Writer
// from a struct with many fields.
type BinWriter struct {
	w   io.Writer
	Err error
	uv  [9]byte
}

// NewBinWriterFromIO makes a BinWriter from io.Writer.
func NewBinWriterFromIO(iow io.Writer) *BinWriter {
	return &BinWriter{w: iow}
}

// WriteU64LE writes a uint64 value into the underlying io.Writer in
// little-endian format.
func (w *BinWriter) WriteU64LE(u64 uint64) {
	binary.LittleEndian.PutUint64(w.uv[:8], u64)
	w.WriteBytes(w.uv[:8])
}

// WriteU32LE writes a uint32 value into the underlying io.Writer in
// little-endian format.
func (w *BinWriter) WriteU32LE(u32 uint32) {
	binary.LittleEndian.PutUint32(w.uv[:4], u32)
	w.WriteBytes(w.uv[:4])
}

// WriteU16LE writes a uint16 value into the underlying io.Writer in
// little-endian format.
func (w *BinWriter) WriteU16LE(u16 uint16) {
	binary.LittleEndian.PutUint16(w.uv[:2], u16)
	w.WriteBytes(w.uv[:2])
}

// WriteU16BE writes a uint16 value into the underlying io.Writer in
// big-endian format.
func (w *BinWriter) WriteU16BE(u16 uint16) {
	binary.BigEndian.PutUint16(w.uv[:2], u16)
	w.WriteBytes(w.uv[:2])
}

// WriteB writes a byte into the underlying io.Writer.
func (w *BinWriter) WriteB(u8 byte) {
	w.uv[0] = u8
	w.WriteBytes(w.uv[:1])
}

// WriteBool writes a boolean value into the underlying io.Writer encoded as
// a byte with values of 0 or 1.
func (w *BinWriter) WriteBool(b bool) {
	var i byte
	if b {
		i = 1
	}
	w.WriteB(i)
}

// WriteArray writes a slice or an array arr into w. Note that nil slices and
// empty slices are gonna be treated the same resulting in an equal zero-length
// array encoded.
//
// Deprecated: Go doesn't support generic methods, but [WriteArray] function
// is much faster that this method.
func (w *BinWriter) WriteArray(arr any) {
	switch val := reflect.ValueOf(arr); val.Kind() {
	case reflect.Slice, reflect.Array:
		if w.Err != nil {
			return
		}

		typ := val.Type().Elem()

		w.WriteVarUint(uint64(val.Len()))
		for i := 0; i < val.Len(); i++ {
			el, ok := val.Index(i).Interface().(encodable)
			if !ok {
				el, ok = val.Index(i).Addr().Interface().(encodable)
				if !ok {
					panic(typ.String() + " is not encodable")
				}
			}

			el.EncodeBinary(w)
		}
	default:
		panic("not an array")
	}
}

// WriteArray writes a slice arr into w. It is a generic-based version of
// [BinWriter.WriteArray] which works much faster.
func WriteArray[Slice ~[]E, E Serializable](w *BinWriter, arr Slice) {
	w.WriteVarUint(uint64(len(arr)))
	for i := range arr {
		arr[i].EncodeBinary(w)
	}
}

// WriteVarUint writes a uint64 into the underlying writer using variable-length encoding.
func (w *BinWriter) WriteVarUint(val uint64) {
	if w.Err != nil {
		return
	}

	n := PutVarUint(w.uv[:], val)
	w.WriteBytes(w.uv[:n])
}

// PutVarUint puts a val in the varint form to the pre-allocated buffer.
func PutVarUint(data []byte, val uint64) int {
	_ = data[8]
	if val < 0xfd {
		data[0] = byte(val)
		return 1
	}
	if val < 0xFFFF {
		data[0] = byte(0xfd)
		binary.LittleEndian.PutUint16(data[1:], uint16(val))
		return 3
	}
	if val < 0xFFFFFFFF {
		data[0] = byte(0xfe)
		binary.LittleEndian.PutUint32(data[1:], uint32(val))
		return 5
	}

	data[0] = byte(0xff)
	binary.LittleEndian.PutUint64(data[1:], val)
	return 9
}

// WriteBytes writes a variable byte into the underlying io.Writer without prefix.
func (w *BinWriter) WriteBytes(b []byte) {
	if w.Err != nil {
		return
	}
	_, w.Err = w.w.Write(b)
}

// WriteVarBytes writes a variable length byte array into the underlying io.Writer.
func (w *BinWriter) WriteVarBytes(b []byte) {
	w.WriteVarUint(uint64(len(b)))
	w.WriteBytes(b)
}

// WriteString writes a variable length string into the underlying io.Writer.
func (w *BinWriter) WriteString(s string) {
	w.WriteVarUint(uint64(len(s)))
	if w.Err != nil {
		return
	}
	_, w.Err = io.WriteString(w.w, s)
}

// Grow tries to increase the underlying buffer capacity so that at least n bytes
// can be written without reallocation. If the writer is not a buffer, this is a no-op.
func (w *BinWriter) Grow(n int) {
	if b, ok := w.w.(*bytes.Buffer); ok {
		b.Grow(n)
	}
}
