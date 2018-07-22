package wire

import (
	"bytes"
	"encoding/binary"
	"io"
)

func calculatePayloadLength(buf *bytes.Buffer) uint32 {

	return uint32(buf.Len())
}
func calculateCheckSum(buf *bytes.Buffer) uint32 {

	checksum := sumSHA256(sumSHA256(buf.Bytes()))
	return binary.LittleEndian.Uint32(checksum[:4])
}

/// Binary writer

type binWriter struct {
	w   io.Writer
	err error
}

func (w *binWriter) Write(v interface{}) {
	if w.err != nil {
		return
	}
	binary.Write(w.w, binary.LittleEndian, v)
}

// Only used for IP and PORT. Additional method
// makes the default LittleEndian case clean
func (w *binWriter) WriteBigEnd(v interface{}) {
	if w.err != nil {
		return
	}
	binary.Write(w.w, binary.BigEndian, v)
}
