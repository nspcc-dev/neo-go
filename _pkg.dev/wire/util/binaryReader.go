package util

import (
	"encoding/binary"
	"io"
)

//BinReader is a convenient wrapper around a io.Reader and err object
// Used to simplify error handling when reading into a struct with many fields
type BinReader struct {
	R   io.Reader
	Err error
}

// Read reads from the underlying io.Reader
// into the interface v in LE
func (r *BinReader) Read(v interface{}) {
	if r.Err != nil {
		return
	}
	r.Err = binary.Read(r.R, binary.LittleEndian, v)
}

// ReadBigEnd reads from the underlying io.Reader
// into the interface v in BE
func (r *BinReader) ReadBigEnd(v interface{}) {
	if r.Err != nil {
		return
	}
	r.Err = binary.Read(r.R, binary.BigEndian, v)
}

//VarUint reads a variable integer from the
// underlying reader
func (r *BinReader) VarUint() uint64 {
	var b uint8
	r.Err = binary.Read(r.R, binary.LittleEndian, &b)

	if b == 0xfd {
		var v uint16
		r.Err = binary.Read(r.R, binary.LittleEndian, &v)
		return uint64(v)
	}
	if b == 0xfe {
		var v uint32
		r.Err = binary.Read(r.R, binary.LittleEndian, &v)
		return uint64(v)
	}
	if b == 0xff {
		var v uint64
		r.Err = binary.Read(r.R, binary.LittleEndian, &v)
		return v
	}

	return uint64(b)
}

// VarBytes reads the next set of bytes from the underlying reader.
// VarUInt is used to determine how large that slice is
func (r *BinReader) VarBytes() []byte {
	n := r.VarUint()
	b := make([]byte, n)
	r.Read(b)
	return b
}

// VarString calls VarBytes and casts the results as a string
func (r *BinReader) VarString() string {
	b := r.VarBytes()
	return string(b)
}
