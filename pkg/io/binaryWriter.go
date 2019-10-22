package io

import (
	"encoding/binary"
	"io"
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

// WriteBytes writes a variable length byte array into the underlying io.Writer.
func (w *BinWriter) WriteBytes(b []byte) {
	w.WriteVarUint(uint64(len(b)))
	w.WriteLE(b)
}

// WriteString writes a variable length string into the underlying io.Writer.
func (w *BinWriter) WriteString(s string) {
	w.WriteBytes([]byte(s))
}
