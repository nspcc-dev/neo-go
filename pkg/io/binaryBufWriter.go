package io

import (
	"bytes"
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
	*BinWriter
	buf *bytes.Buffer
}

// NewBufBinWriter makes a BufBinWriter with an empty byte buffer.
func NewBufBinWriter() BufferWriter {
	b := new(bytes.Buffer)
	return &BufBinWriter{BinWriter: NewBinWriterFromIO(b), buf: b}
}

// Len returns the number of bytes of the unread portion of the buffer.
func (bw *BufBinWriter) Len() int {
	return bw.buf.Len()
}

// Bytes returns resulting buffer and makes future writes return an error.
func (bw *BufBinWriter) Bytes() []byte {
	if bw.Err != nil {
		return nil
	}
	bw.Err = errors.New("buffer already drained")
	return bw.buf.Bytes()
}

// Reset resets the state of the buffer, making it usable again. It can
// make buffer usage somewhat more efficient, because you don't need to
// create it again, but beware that the buffer is gonna be the same as the one
// returned by Bytes(), so if you need that data after Reset() you have to copy
// it yourself.
func (bw *BufBinWriter) Reset() {
	bw.Err = nil
	bw.buf.Reset()
}
