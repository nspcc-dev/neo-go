package wire

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
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
	// bw.Write(message.PayloadLength())
	// bw.Write(message.Checksum())
	buf := new(bytes.Buffer)
	if err := message.EncodePayload(buf); err != nil {
		return err
	}

	payloadLen := util.CalculatePayloadLength(buf)
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

	fmt.Println("Command is", header.CMD)
	fmt.Println("payload lengthA is ", header.PayloadLength)
	fmt.Println("payload length is ", len(buf.Bytes()))

	fmt.Println("Newly cal checksum", Hash256(buf.Bytes())[:4])
	fmt.Println("Header checksum", header.Checksum)
	// Compare the checksum of the payload.
	// if !checksum.Compare(header.Checksum, buf.Bytes()) {
	// 	return nil, errChecksumMismatch
	// }
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
	case command.Inv:
		v := &payload.InvMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		return v, nil
	case command.GetAddr:
		v := &payload.GetAddrMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		return v, nil
	case command.Addr:
		v := &payload.AddrMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		return v, nil
	case command.Block:
		v := &payload.BlockMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
	case command.GetBlocks:
		v := &payload.GetBlocksMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		return v, nil
	case command.GetData:
		v := &payload.GetDataMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		return v, nil
	case command.GetHeaders:
		v := &payload.GetHeadersMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		return v, nil
	case command.Headers:
		v := &payload.HeadersMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		fmt.Println("Direct checksum", v.Checksum())
		return v, nil
	case command.TX:
		reader := bufio.NewReader(buf)
		tx, err := transaction.FromBytes(reader)

		if err != nil {
			return nil, err
		}
		t, err := payload.NewTXMessage(tx)

		return t, err
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

func calcHash(buf []byte, hasher hash.Hash) []byte {
	hasher.Write(buf)
	return hasher.Sum(nil)
}

// Hash160 calculates the hash ripemd160(sha256(b)).
func Hash256(buf []byte) []byte {
	return calcHash(calcHash(buf, sha256.New()), sha256.New())
}
