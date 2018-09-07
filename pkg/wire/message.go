package wire

import (
	"bufio"
	"bytes"
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	checksum "github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Messager interface {
	EncodePayload(w io.Writer) error
	DecodePayload(r io.Reader) error
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

	buf := new(bytes.Buffer)
	if err := message.EncodePayload(buf); err != nil {
		return err
	}

	payloadLen := util.BufferLength(buf)
	checksum := checksum.FromBytes(buf.Bytes())

	bw.Write(payloadLen)
	bw.Write(checksum)

	bw.WriteBigEnd(buf.Bytes())

	return bw.Err
}

func ReadMessage(r io.Reader, magic protocol.Magic) (Messager, error) {

	byt := make([]byte, minMsgSize)

	if _, err := io.ReadFull(r, byt); err != nil {
		return nil, err
	}

	reader := bytes.NewReader(byt)

	var header Base
	_, err := header.DecodeBase(reader)

	if err != nil {
		return nil, errors.New("Error decoding into the header base")
	}

	buf := new(bytes.Buffer)

	_, err = io.CopyN(buf, r, int64(header.PayloadLength))
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
		err := v.DecodePayload(buf)
		return v, err
	case command.Verack:
		v, err := payload.NewVerackMessage()
		err = v.DecodePayload(buf)
		return v, err
	case command.Inv:
		v, err := payload.NewInvMessage(0)
		err = v.DecodePayload(buf)
		return v, err
	case command.GetAddr:
		v, err := payload.NewGetAddrMessage()
		err = v.DecodePayload(buf)
		return v, err
	case command.Addr:
		v, err := payload.NewAddrMessage()
		err = v.DecodePayload(buf)
		return v, err
	case command.Block:
		v, err := payload.NewBlockMessage()
		err = v.DecodePayload(buf)
		return v, err
	case command.GetBlocks:
		v, err := payload.NewGetBlocksMessage([]util.Uint256{}, util.Uint256{})
		err = v.DecodePayload(buf)
		return v, err
	case command.GetData:
		v, err := payload.NewGetDataMessage(payload.InvTypeTx)
		err = v.DecodePayload(buf)
		return v, err
	case command.GetHeaders:
		v, err := payload.NewGetHeadersMessage([]util.Uint256{}, util.Uint256{})
		err = v.DecodePayload(buf)
		return v, err
	case command.Headers:
		v, err := payload.NewHeadersMessage()
		err = v.DecodePayload(buf)
		return v, err
	case command.TX:
		reader := bufio.NewReader(buf)
		tx, err := transaction.FromBytes(reader)
		if err != nil {
			return nil, err
		}
		return payload.NewTXMessage(tx)
	}
	return nil, errors.New("Unknown Message found")

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
