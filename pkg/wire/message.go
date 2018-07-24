package wire

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Messager interface {
	EncodePayload(w io.Writer) error
	DecodePayload(r io.Reader) error
	PayloadLength() uint32
	Checksum() uint32
	Command() command.Type
}

var (
	errChecksumMismatch = errors.New("checksum mismatch")
)

func WriteMessage(w io.Writer, magic protocol.Magic, message Messager) error {
	bw := &util.BinWriter{W: w}
	bw.Write(magic)
	bw.Write(cmdToByteArray(message.Command()))
	bw.Write(message.PayloadLength())
	bw.Write(message.Checksum())

	if err := message.EncodePayload(bw.W); err != nil {
		return err
	}

	return bw.Err
}

func ReadMessage(r io.Reader, magic protocol.Magic) (Messager, error) {

	var header Base
	r, err := header.DecodeBase(r)

	buf := new(bytes.Buffer)

	n, err := io.CopyN(buf, r, int64(header.Length))
	if err != nil {
		return nil, err
	}

	if uint32(n) != header.Length {
		return nil, fmt.Errorf("expected to have read exactly %d bytes got %d", header.Length, n)
	}
	// Compare the checksum of the payload.
	if !util.CompareChecksum(header.Checksum, buf.Bytes()) {
		return nil, errChecksumMismatch
	}
	switch header.CMD {
	case command.Version:
		v := &payload.VersionMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		return v, nil
	}
	return nil, nil

}

func cmdToByteArray(cmd command.Type) [command.Size]byte {
	cmdLen := len(cmd)
	if cmdLen > command.Size {
		panic("exceeded command max length of size 12")
	}

	// The command can have max 12 bytes, rest is filled with 0.
	b := [command.Size]byte{}
	for i := 0; i < cmdLen; i++ {
		b[i] = cmd[i]
	}

	return b
}
