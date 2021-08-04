package io

import (
	"encoding/binary"
	"fmt"
	"io"
)

// BufferWriter is similar to binary reader but also allows to extract
// current position and read portion of data.
type BufBinReader struct {
	Data []byte
	Pos  int
	Err  error
}

// NewBinReaderFromBuf makes a BinReader from byte buffer.
func NewBinReaderFromBuf(b []byte) *BufBinReader {
	return &BufBinReader{
		Data: b,
	}
}

// ReadU64LE implements BinaryReader interface.
func (r *BufBinReader) ReadU64LE() uint64 {
	if r.Err == nil {
		if pos := r.Pos; pos+8 <= len(r.Data) {
			r.Pos += 8
			return binary.LittleEndian.Uint64(r.Data[pos:])
		}
		r.Err = io.EOF
	}
	return 0
}

// ReadU32LE implements BinaryReader interface.
func (r *BufBinReader) ReadU32LE() uint32 {
	if r.Err == nil {
		if pos := r.Pos; pos+4 <= len(r.Data) {
			r.Pos += 4
			return binary.LittleEndian.Uint32(r.Data[pos:])
		}
		r.Err = io.EOF
	}
	return 0
}

// ReadU16LE implements BinaryReader interface.
func (r *BufBinReader) ReadU16LE() uint16 {
	if r.Err == nil {
		if pos := r.Pos; pos+2 <= len(r.Data) {
			r.Pos += 2
			return binary.LittleEndian.Uint16(r.Data[pos:])
		}
		r.Err = io.EOF
	}
	return 0
}

// ReadU16BE implements BinaryReader interface.
func (r *BufBinReader) ReadU16BE() uint16 {
	if r.Err == nil {
		if pos := r.Pos; pos+2 <= len(r.Data) {
			r.Pos += 2
			return binary.BigEndian.Uint16(r.Data[pos:])
		}
		r.Err = io.EOF
	}
	return 0
}

// ReadB implements BinaryReader interface.
func (r *BufBinReader) ReadB() byte {
	if r.Err == nil {
		if pos := r.Pos; pos < len(r.Data) {
			r.Pos++
			return r.Data[pos]
		}
		r.Err = io.EOF
	}
	return 0
}

// ReadBool implements BinaryReader interface.
func (r *BufBinReader) ReadBool() bool {
	return r.ReadB() != 0
}

// ReadArray implements BinaryReader interface.
func (r *BufBinReader) ReadArray(t interface{}, maxSize ...int) {
	readArray(r, t, maxSize...)
}

// ReadVarUint implements BinaryReader interface.
func (r *BufBinReader) ReadVarUint() uint64 {
	if r.Err != nil {
		return 0
	}

	var b = r.ReadB()

	if b == 0xfd {
		return uint64(r.ReadU16LE())
	}
	if b == 0xfe {
		return uint64(r.ReadU32LE())
	}
	if b == 0xff {
		return r.ReadU64LE()
	}

	return uint64(b)
}

// ReadVarBytes implements BinaryReader interface.
func (r *BufBinReader) ReadVarBytes(maxSize ...int) []byte {
	n := r.ReadVarUint()
	ms := MaxArraySize
	if len(maxSize) != 0 {
		ms = maxSize[0]
	}
	if n > uint64(ms) {
		r.Err = fmt.Errorf("byte-slice is too big (%d)", n)
		return nil
	}
	b := make([]byte, n)
	r.ReadBytes(b)
	return b
}

// ReadBytes implements BinaryReader interface.
func (r *BufBinReader) ReadBytes(bytes []byte) {
	if r.Err != nil {
		return
	}

	n := copy(bytes, r.Data[r.Pos:])
	r.Pos += n
	if n < len(bytes) {
		if n == 0 {
			r.Err = io.EOF
		} else {
			r.Err = io.ErrUnexpectedEOF
		}
	}
	return
}

// ReadString implements BinaryReader interface.
func (r *BufBinReader) ReadString(maxSize ...int) string {
	n := r.ReadVarUint()
	ms := MaxArraySize
	if len(maxSize) != 0 {
		ms = maxSize[0]
	}
	if n > uint64(ms) {
		if r.Err == nil {
			r.Err = fmt.Errorf("byte-slice is too big (%d)", n)
		}
		return ""
	}
	pos := r.Pos
	r.Pos += int(n)
	return string(r.Data[pos:r.Pos])
}

// Error implements BinaryReader interface.
func (r *BufBinReader) Error() error {
	return r.Err
}

// SetError implements BinaryReader interface.
func (r *BufBinReader) SetError(err error) {
	r.Err = err
}
