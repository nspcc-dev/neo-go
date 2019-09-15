package util

import (
	"bytes"
	"errors"
)

// BufBinWriter is an additional layer on top of BinWriter that
// automatically creates buffer to write into that you can get after all
// writes via Bytes().
type BufBinWriter struct {
	*BinWriter
	buf *bytes.Buffer
}

// NewBufBinWriter makes a BufBinWriter with an empty byte buffer.
func NewBufBinWriter() *BufBinWriter {
	b := new(bytes.Buffer)
	return &BufBinWriter{BinWriter: NewBinWriterFromIO(b), buf: b}
}

// Bytes returns resulting buffer and makes future writes return an error.
func (bw *BufBinWriter) Bytes() []byte {
	if bw.Err != nil {
		return nil
	}
	bw.Err = errors.New("buffer already drained")
	return bw.buf.Bytes()
}
