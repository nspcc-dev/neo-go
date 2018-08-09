package wire

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Base is everything in the message except the payload

type Base struct {
	Magic         uint32
	CMD           command.Type
	PayloadLength uint32
	Checksum      uint32
}

// Note, That there is no EncodeBase
// As the header is implicitly inferred from
// the message on Encode To send
func (h *Base) DecodeBase(r io.Reader) (io.Reader, error) {
	br := &util.BinReader{R: r}

	br.Read(&h.Magic)

	var cmd [12]byte
	br.Read(&cmd)
	h.CMD = command.Type(cmdByteArrayToString(cmd))

	br.Read(&h.PayloadLength)
	br.Read(&h.Checksum)
	return br.R, br.Err
}

func cmdByteArrayToString(cmd [command.Size]byte) string {
	buf := []byte{}
	for i := 0; i < command.Size; i++ {
		if cmd[i] != 0 {
			buf = append(buf, cmd[i])
		}
	}
	return string(buf)
}
