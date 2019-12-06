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
	Err error
}

// NewBinWriterFromIO makes a BinWriter from io.Writer.
func NewBinWriterFromIO(iow io.Writer) *BinWriter {
	return &BinWriter{w: iow}
}

// WriteLE writes into the underlying io.Writer from an object v in little-endian format.
func (w *BinWriter) WriteLE(v interface{}) {
	if w.Err != nil {
		return
	}
	w.Err = binary.Write(w.w, binary.LittleEndian, v)
}

// WriteBE writes into the underlying io.Writer from an object v in big-endian format.
func (w *BinWriter) WriteBE(v interface{}) {
	if w.Err != nil {
		return
	}
	w.Err = binary.Write(w.w, binary.BigEndian, v)
}

// WriteArray writes a slice or an array arr into w.
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
				panic(typ.String() + "is not encodable")
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

	if val < 0xfd {
		w.Err = binary.Write(w.w, binary.LittleEndian, uint8(val))
		return
	}
	if val < 0xFFFF {
		w.Err = binary.Write(w.w, binary.LittleEndian, byte(0xfd))
		w.Err = binary.Write(w.w, binary.LittleEndian, uint16(val))
		return
	}
	if val < 0xFFFFFFFF {
		w.Err = binary.Write(w.w, binary.LittleEndian, byte(0xfe))
		w.Err = binary.Write(w.w, binary.LittleEndian, uint32(val))
		return

	}

	w.Err = binary.Write(w.w, binary.LittleEndian, byte(0xff))
	w.Err = binary.Write(w.w, binary.LittleEndian, val)

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
