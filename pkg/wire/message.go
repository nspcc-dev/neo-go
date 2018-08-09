package wire

import (
	"bytes"
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"

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

const (
	// Magic + cmd + length + checksum
	minMsgSize = 4 + 12 + 4 + 4
)

var (
	errChecksumMismatch = errors.New("checksum mismatch")
)

func WriteMessage(w io.Writer, magic protocol.Magic, message Messager) error {
	bw := &util.BinWriter{W: w}
	bw.Write(magic)
	bw.Write(cmdToByteArray(message.Command()))
	bw.Write(message.PayloadLength())
	bw.Write(message.Checksum())

	buf := new(bytes.Buffer)
	if err := message.EncodePayload(buf); err != nil {
		return err
	}

	bw.WriteBigEnd(buf.Bytes())

	return bw.Err
}

func ReadMessage(r io.Reader, magic protocol.Magic) (Messager, error) {

	var header Base
	rem, err := header.DecodeBase(r)

	buf := new(bytes.Buffer)

	_, err = io.CopyN(buf, rem, int64(header.PayloadLength))
	if err != nil {
		return nil, err
	}

	// Compare the checksum of the payload.
	if !checksum.Compare(header.Checksum, buf.Bytes()) {
		return nil, errChecksumMismatch
	}
	switch header.CMD {
	case command.Version:
		v := &payload.VersionMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		return v, nil
	case command.Verack:
		v := &payload.VerackMessage{}
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
