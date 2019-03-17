package util

import (
	"encoding/binary"
	"errors"
	"io"
)

//BinWriter is a convenient wrapper around a io.Writer and err object
// Used to simplify error handling when writing into a io.Writer
// from a struct with many fields
type BinWriter struct {
	W   io.Writer
	Err error
}

// Write writes into the underlying io.Writer from an object v in LE format
func (w *BinWriter) Write(v interface{}) {
	if w.Err != nil {
		return
	}
	w.Err = binary.Write(w.W, binary.LittleEndian, v)
}

// WriteBigEnd writes into the underlying io.Writer from an object v in BE format
// Only used for IP and PORT. Additional method makes the default LittleEndian case clean
func (w *BinWriter) WriteBigEnd(v interface{}) {
	if w.Err != nil {
		return
	}
	w.Err = binary.Write(w.W, binary.BigEndian, v)
}

// VarUint writes a uint64 into the underlying writer
func (w *BinWriter) VarUint(val uint64) {
	if val < 0 {
		w.Err = errors.New("value out of range")
		return
	}

	if w.Err != nil {
		return
	}

	if val < 0xfd {
		w.Err = binary.Write(w.W, binary.LittleEndian, uint8(val))
		return
	}
	if val < 0xFFFF {
		w.Err = binary.Write(w.W, binary.LittleEndian, byte(0xfd))
		w.Err = binary.Write(w.W, binary.LittleEndian, uint16(val))
		return
	}
	if val < 0xFFFFFFFF {
		w.Err = binary.Write(w.W, binary.LittleEndian, byte(0xfe))
		w.Err = binary.Write(w.W, binary.LittleEndian, uint32(val))
		return

	}

	w.Err = binary.Write(w.W, binary.LittleEndian, byte(0xff))
	w.Err = binary.Write(w.W, binary.LittleEndian, val)

}

// VarBytes writes a variable length byte array into the underlying io.Writer
func (w *BinWriter) VarBytes(b []byte) {
	w.VarUint(uint64(len(b)))
	w.Write(b)
}

//VarString casts the string as a byte slice and calls VarBytes
func (w *BinWriter) VarString(s string) {
	w.VarBytes([]byte(s))
}
