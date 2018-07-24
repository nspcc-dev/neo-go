package wire

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// MessageBase is everything in the message except the payload

type MessageBase struct {
	Magic    uint32
	Command  CommandType
	Length   uint32
	Checksum uint32
}

// Note, That there is no EncodeMessageHeader
// As the header is implicitly inferred from
// the message on encode
func (h *MessageBase) DecodeMessageBase(r io.Reader) (io.Reader, error) {
	br := &util.BinReader{R: r}

	br.Read(&h.Magic)

	var command [12]byte
	br.Read(&command)
	h.Command = CommandType(cmdByteArrayToString(command))

	br.Read(&h.Length)
	br.Read(&h.Checksum)
	return br.R, br.Err
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
