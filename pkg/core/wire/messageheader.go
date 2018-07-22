package wire

import (
	"fmt"
	"io"
)

type MessageHeader struct {
	Magic    uint32
	Command  CommandType
	Length   uint32
	Checksum uint32
}

// Note, That there is no EncodeMessageHeader
// As the header is implicitly inferred from
// the message on encode
func (h *MessageHeader) DecodeMessageHeader(r io.Reader) (io.Reader, error) {
	br := &binReader{r: r}
	br.Read(&h.Magic)
	fmt.Println(h.Magic)

	var command [12]byte
	br.Read(&command)
	h.Command = CommandType(cmdByteArrayToString(command))
	fmt.Println(h.Command)
	br.Read(&h.Length)
	fmt.Println(h.Length)
	br.Read(&h.Checksum)
	fmt.Println(h.Checksum)
	return br.r, br.err
}

func cmdByteArrayToString(cmd [cmdSize]byte) string {
	buf := []byte{}
	for i := 0; i < cmdSize; i++ {
		if cmd[i] != 0 {
			buf = append(buf, cmd[i])
		}
	}
	return string(buf)
}
