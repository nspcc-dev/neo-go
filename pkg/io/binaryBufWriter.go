package io

import (
	"encoding/binary"
	"errors"
)

// BufferWriter is similar to binary writer but also allows written
// bytes to be extracted in the byte-slice and later reuse the underlying buffer.
type BufferWriter interface {
	BinaryWriter
	Reset()
	Len() int
	Bytes() []byte
}

// BufBinWriter is an additional layer on top of BinWriter that
// automatically creates buffer to write into that you can get after all
// writes via Bytes().
type BufBinWriter struct {
	buf []byte
	err error
}

// NewBufBinWriter makes a BufBinWriter with an empty byte buffer.
func NewBufBinWriter() *BufBinWriter {
	return new(BufBinWriter)
}

// Len returns the number of bytes of the unread portion of the buffer.
func (bw *BufBinWriter) Len() int {
	return len(bw.buf)
}

// Bytes returns resulting buffer and makes future writes return an error.
func (bw *BufBinWriter) Bytes() []byte {
	if bw.err != nil {
		return nil
	}
	bw.err = errors.New("buffer already drained")
	return bw.buf
}

// Error implements BinaryWriter interface.
func (bw *BufBinWriter) Error() error { return bw.err }

// SetError implements BinaryWriter interface.
func (bw *BufBinWriter) SetError(err error) { bw.err = err }

// Reset resets the state of the buffer, making it usable again. It can
// make buffer usage somewhat more efficient, because you don't need to
// create it again, but beware that the buffer is gonna be the same as the one
// returned by Bytes(), so if you need that data after Reset() you have to copy
// it yourself.
func (bw *BufBinWriter) Reset() {
	bw.err = nil
	bw.buf = bw.buf[:0]
}

// WriteU64LE implements BinaryWriter interface.
func (bw *BufBinWriter) WriteU64LE(v uint64) {
	n := bw.grow(8)
	binary.LittleEndian.PutUint64(bw.buf[n:], v)
}

// WriteU32LE implements BinaryWriter interface.
func (bw *BufBinWriter) WriteU32LE(v uint32) {
	n := bw.grow(4)
	binary.LittleEndian.PutUint32(bw.buf[n:], v)
}

// WriteU16LE implements BinaryWriter interface.
func (bw *BufBinWriter) WriteU16LE(v uint16) {
	n := bw.grow(2)
	binary.LittleEndian.PutUint16(bw.buf[n:], v)
}

// WriteU16BE implements BinaryWriter interface.
func (bw *BufBinWriter) WriteU16BE(v uint16) {
	n := bw.grow(2)
	binary.BigEndian.PutUint16(bw.buf[n:], v)
}

// WriteB implements BinaryWriter interface.
func (bw *BufBinWriter) WriteB(v byte) {
	n := bw.grow(1)
	bw.buf[n] = v
}

// WriteBool implements BinaryWriter interface.
func (bw *BufBinWriter) WriteBool(v bool) {
	var num byte
	if v {
		num = 1
	}
	bw.WriteB(num)
}

// WriteArray implements BinaryWriter interface.
func (bw *BufBinWriter) WriteArray(arr interface{}) {
	writeArray(bw, arr)
}

// WriteVarUint implements BinaryWriter interface.
func (bw *BufBinWriter) WriteVarUint(v uint64) {
	switch {
	case v < 0xfd:
		bw.WriteB(byte(v))
	case v < 0xFFFF:
		n := bw.grow(3)
		bw.buf[n] = 0xfd
		binary.LittleEndian.PutUint16(bw.buf[n+1:], uint16(v))
	case v < 0xFFFFFFFF:
		n := bw.grow(5)
		bw.buf[n] = 0xfe
		binary.LittleEndian.PutUint32(bw.buf[n+1:], uint32(v))
	default:
		n := bw.grow(9)
		bw.buf[n] = 0xff
		binary.LittleEndian.PutUint64(bw.buf[n+1:], uint64(v))
	}
}

// WriteBytes implements BinaryWriter interface.
func (bw *BufBinWriter) WriteBytes(v []byte) {
	if bw.err != nil {
		return
	}
	n := bw.grow(len(v))
	copy(bw.buf[n:], v)
}

// WriteVarBytes implements BinaryWriter interface.
func (bw *BufBinWriter) WriteVarBytes(v []byte) {
	bw.WriteVarUint(uint64(len(v)))
	n := bw.grow(len(v))
	copy(bw.buf[n:], v)
}

// WriteString implements BinaryWriter interface.
func (bw *BufBinWriter) WriteString(s string) {
	bw.WriteVarUint(uint64(len(s)))
	n := bw.grow(len(s))
	copy(bw.buf[n:], s)
}

func (bw *BufBinWriter) grow(n int) int {
	ln := len(bw.buf)
	if ln+n < cap(bw.buf) {
		bw.buf = bw.buf[:ln+n]
		return ln
	}

	buf := make([]byte, cap(bw.buf)*2+n)
	copy(buf, bw.buf)
	bw.buf = buf[:ln+n]
	return ln
}
