package io

import (
	"encoding/binary"
	"io"
	"reflect"
)

// BinWriter is a convenient wrapper around a io.Writer and err object.
// Used to simplify error handling when writing into a io.Writer
// from a struct with many fields.
type BinWriter struct {
	w   io.Writer
	uv  []byte
	u64 []byte
	u32 []byte
	u16 []byte
	u8  []byte
	Err error
}

// NewBinWriterFromIO makes a BinWriter from io.Writer.
func NewBinWriterFromIO(iow io.Writer) *BinWriter {
	uv := make([]byte, 9)
	u64 := uv[:8]
	u32 := u64[:4]
	u16 := u64[:2]
	u8 := u64[:1]
	return &BinWriter{w: iow, uv: uv, u64: u64, u32: u32, u16: u16, u8: u8}
}

// WriteU64LE writes an uint64 value into the underlying io.Writer in
// little-endian format.
func (w *BinWriter) WriteU64LE(u64 uint64) {
	binary.LittleEndian.PutUint64(w.u64, u64)
	w.WriteBytes(w.u64)
}

// WriteU32LE writes an uint32 value into the underlying io.Writer in
// little-endian format.
func (w *BinWriter) WriteU32LE(u32 uint32) {
	binary.LittleEndian.PutUint32(w.u32, u32)
	w.WriteBytes(w.u32)
}

// WriteU16LE writes an uint16 value into the underlying io.Writer in
// little-endian format.
func (w *BinWriter) WriteU16LE(u16 uint16) {
	binary.LittleEndian.PutUint16(w.u16, u16)
	w.WriteBytes(w.u16)
}

// WriteU16BE writes an uint16 value into the underlying io.Writer in
// big-endian format.
func (w *BinWriter) WriteU16BE(u16 uint16) {
	binary.BigEndian.PutUint16(w.u16, u16)
	w.WriteBytes(w.u16)
}

// WriteB writes a byte into the underlying io.Writer.
func (w *BinWriter) WriteB(u8 byte) {
	w.u8[0] = u8
	w.WriteBytes(w.u8)
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
// empty slices are gonna be treated the same resulting in equal zero-length
// array encoded.
func (w *BinWriter) WriteArray(arr interface{}) {
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

// WriteVarUint writes a uint64 into the underlying writer using variable-length encoding.
func (w *BinWriter) WriteVarUint(val uint64) {
	if w.Err != nil {
		return
	}

	n := PutVarUint(w.uv, val)
	w.WriteBytes(w.uv[:n])
}

// PutVarUint puts val in varint form to the pre-allocated buffer.
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
	w.WriteVarBytes([]byte(s))
}
