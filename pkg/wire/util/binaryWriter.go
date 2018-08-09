package util

import (
	"encoding/binary"
	"errors"
	"io"
)

type BinWriter struct {
	W   io.Writer
	Err error
}

func (w *BinWriter) Write(v interface{}) {
	if w.Err != nil {
		return
	}
	w.Err = binary.Write(w.W, binary.LittleEndian, v)
}

// Only used for IP and PORT. Additional method makes the default LittleEndian case clean
func (w *BinWriter) WriteBigEnd(v interface{}) {
	if w.Err != nil {
		return
	}
	w.Err = binary.Write(w.W, binary.BigEndian, v)
}

func (w *BinWriter) VarString(s string) {
	w.VarBytes([]byte(s))
}

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

// WriteVarBytes writes a variable length byte array.
func (w *BinWriter) VarBytes(b []byte) {
	w.VarUint(uint64(len(b)))
	w.Write(b)
}
