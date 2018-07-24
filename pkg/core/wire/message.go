package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/wire/util"
)

type Messager interface {
	EncodePayload(w io.Writer) error
	DecodePayload(r io.Reader) error
	PayloadLength() uint32
	Checksum() uint32
	Command() CommandType
}

var (
	errChecksumMismatch = errors.New("checksum mismatch")
)

// CommandType represents the type of a message command.
type CommandType string

// Valid protocol commands used to send between nodes.
// use this to get
const (
	CMDVersion    CommandType = "version"
	CMDPing       CommandType = "ping"
	CMDPong       CommandType = "pong"
	CMDVerack     CommandType = "verack"
	CMDGetAddr    CommandType = "getaddr"
	CMDAddr       CommandType = "addr"
	CMDGetHeaders CommandType = "getheaders"
	CMDHeaders    CommandType = "headers"
	CMDGetBlocks  CommandType = "getblocks"
	CMDInv        CommandType = "inv"
	CMDGetData    CommandType = "getdata"
	CMDBlock      CommandType = "block"
	CMDTX         CommandType = "tx"
	CMDConsensus  CommandType = "consensus"
	CMDUnknown    CommandType = "unknown"
)

func WriteMessage(w io.Writer, magic Magic, message Messager) error {
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

func ReadMessage(r io.Reader, magic Magic) (Messager, error) {

	var header MessageHeader
	r, err := header.DecodeMessageHeader(r)

	buf := new(bytes.Buffer)

	n, err := io.CopyN(buf, r, int64(header.Length))
	if err != nil {
		return nil, err
	}

	if uint32(n) != header.Length {
		return nil, fmt.Errorf("expected to have read exactly %d bytes got %d", header.Length, n)
	}
	// Compare the checksum of the payload.
	if !compareChecksum(header.Checksum, buf.Bytes()) {
		return nil, errChecksumMismatch
	}
	switch header.Command {
	case CMDVersion:
		v := &VersionMessage{}
		if err := v.DecodePayload(buf); err != nil {
			return nil, err
		}
		fmt.Println("We have decoded", v)
		return v, nil
	}
	return nil, nil

}

func cmdToByteArray(cmd CommandType) [cmdSize]byte {
	cmdLen := len(cmd)
	if cmdLen > cmdSize {
		panic("exceeded command max length of size 12")
	}

	// The command can have max 12 bytes, rest is filled with 0.
	b := [cmdSize]byte{}
	for i := 0; i < cmdLen; i++ {
		b[i] = cmd[i]
	}

	return b
}

func sumSHA256(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)
}

func compareChecksum(have uint32, b []byte) bool {
	sum := sumSHA256(sumSHA256(b))[:4]
	want := binary.LittleEndian.Uint32(sum)
	return have == want
}
