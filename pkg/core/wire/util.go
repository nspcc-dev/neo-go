package wire

import (
	"bytes"
	"encoding/binary"
)

func calculatePayloadLength(buf *bytes.Buffer) uint32 {

	return uint32(buf.Len())
}
func calculateCheckSum(buf *bytes.Buffer) uint32 {

	checksum := sumSHA256(sumSHA256(buf.Bytes()))
	return binary.LittleEndian.Uint32(checksum[:4])
}
