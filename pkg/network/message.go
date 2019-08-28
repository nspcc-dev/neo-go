package network

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

const (
	// The minimum size of a valid message.
	minMessageSize = 24
	cmdSize        = 12
)

var (
	errChecksumMismatch = errors.New("checksum mismatch")
)

// Message is the complete message send between nodes.
type Message struct {
	// NetMode of the node that sends this message.
	Magic config.NetMode

	// Command is utf8 code, of which the length is 12 bytes,
	// the extra part is filled with 0.
	Command [cmdSize]byte

	// Length of the payload
	Length uint32

	// Checksum is the first 4 bytes of the value that two times SHA256
	// hash of the payload
	Checksum uint32

	// Payload send with the message.
	Payload payload.Payload
}

// CommandType represents the type of a message command.
type CommandType string

// Valid protocol commands used to send between nodes.
const (
	CMDAddr        CommandType = "addr"
	CMDBlock       CommandType = "block"
	CMDConsensus   CommandType = "consensus"
	CMDFilterAdd   CommandType = "filteradd"
	CMDFilterClear CommandType = "filterclear"
	CMDFilterLoad  CommandType = "filterload"
	CMDGetAddr     CommandType = "getaddr"
	CMDGetBlocks   CommandType = "getblocks"
	CMDGetData     CommandType = "getdata"
	CMDGetHeaders  CommandType = "getheaders"
	CMDHeaders     CommandType = "headers"
	CMDInv         CommandType = "inv"
	CMDMempool     CommandType = "mempool"
	CMDMerkleBlock CommandType = "merkleblock"
	CMDPing        CommandType = "ping"
	CMDPong        CommandType = "pong"
	CMDTX          CommandType = "tx"
	CMDUnknown     CommandType = "unknown"
	CMDVerack      CommandType = "verack"
	CMDVersion     CommandType = "version"
)

// NewMessage returns a new message with the given payload.
func NewMessage(magic config.NetMode, cmd CommandType, p payload.Payload) *Message {
	var (
		size     uint32
		checksum []byte
	)

	if p != nil {
		buf := new(bytes.Buffer)
		if err := p.EncodeBinary(buf); err != nil {
			panic(err)
		}
		size = uint32(buf.Len())
		checksum = hash.Checksum(buf.Bytes())
	} else {
		checksum = hash.Checksum([]byte{})
	}

	return &Message{
		Magic:    magic,
		Command:  cmdToByteArray(cmd),
		Length:   size,
		Payload:  p,
		Checksum: binary.LittleEndian.Uint32(checksum[:4]),
	}
}

// CommandType converts the 12 byte command slice to a CommandType.
func (m *Message) CommandType() CommandType {
	cmd := cmdByteArrayToString(m.Command)
	switch cmd {
	case "addr":
		return CMDAddr
	case "block":
		return CMDBlock
	case "consensus":
		return CMDConsensus
	case "filteradd":
		return CMDFilterAdd
	case "filterclear":
		return CMDFilterClear
	case "filterload":
		return CMDFilterLoad
	case "getaddr":
		return CMDGetAddr
	case "getblocks":
		return CMDGetBlocks
	case "getdata":
		return CMDGetData
	case "getheaders":
		return CMDGetHeaders
	case "headers":
		return CMDHeaders
	case "inv":
		return CMDInv
	case "mempool":
		return CMDMempool
	case "merkleblock":
		return CMDMerkleBlock
	case "ping":
		return CMDPing
	case "pong":
		return CMDPong
	case "tx":
		return CMDTX
	case "verack":
		return CMDVerack
	case "version":
		return CMDVersion
	default:
		return CMDUnknown
	}
}

// Decode a Message from the given reader.
func (m *Message) Decode(r io.Reader) error {
	br := util.BinReader{R: r}
	br.ReadLE(&m.Magic)
	br.ReadLE(&m.Command)
	br.ReadLE(&m.Length)
	br.ReadLE(&m.Checksum)
	if br.Err != nil {
		return br.Err
	}
	// return if their is no payload.
	if m.Length == 0 {
		return nil
	}
	return m.decodePayload(r)
}

func (m *Message) decodePayload(r io.Reader) error {
	buf := new(bytes.Buffer)
	n, err := io.CopyN(buf, r, int64(m.Length))
	if err != nil {
		return err
	}

	if uint32(n) != m.Length {
		return fmt.Errorf("expected to have read exactly %d bytes got %d", m.Length, n)
	}

	// Compare the checksum of the payload.
	if !compareChecksum(m.Checksum, buf.Bytes()) {
		return errChecksumMismatch
	}

	var p payload.Payload
	switch m.CommandType() {
	case CMDVersion:
		p = &payload.Version{}
		if err := p.DecodeBinary(buf); err != nil {
			return err
		}
	case CMDInv:
		p = &payload.Inventory{}
		if err := p.DecodeBinary(buf); err != nil {
			return err
		}
	case CMDAddr:
		p = &payload.AddressList{}
		if err := p.DecodeBinary(buf); err != nil {
			return err
		}
	case CMDBlock:
		p = &core.Block{}
		if err := p.DecodeBinary(buf); err != nil {
			return err
		}
	case CMDGetHeaders:
		p = &payload.GetBlocks{}
		if err := p.DecodeBinary(buf); err != nil {
			return err
		}
	case CMDHeaders:
		p = &payload.Headers{}
		if err := p.DecodeBinary(buf); err != nil {
			return err
		}
	case CMDTX:
		p = &transaction.Transaction{}
		if err := p.DecodeBinary(buf); err != nil {
			return err
		}
	case CMDMerkleBlock:
		p = &payload.MerkleBlock{}
		if err := p.DecodeBinary(buf); err != nil {
			return err
		}
	}

	m.Payload = p

	return nil
}

// Encode a Message to any given io.Writer.
func (m *Message) Encode(w io.Writer) error {
	br := util.BinWriter{W: w}
	br.WriteLE(m.Magic)
	br.WriteLE(m.Command)
	br.WriteLE(m.Length)
	br.WriteLE(m.Checksum)
	if br.Err != nil {
		return br.Err
	}
	if m.Payload != nil {
		return m.Payload.EncodeBinary(w)
	}
	return nil
}

// convert a command (string) to a byte slice filled with 0 bytes till
// size 12.
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

func cmdByteArrayToString(cmd [cmdSize]byte) string {
	buf := make([]byte, 0, cmdSize)
	for i := 0; i < cmdSize; i++ {
		if cmd[i] != 0 {
			buf = append(buf, cmd[i])
		}
	}
	return string(buf)
}

func compareChecksum(have uint32, b []byte) bool {
	sum := hash.Checksum(b)
	want := binary.LittleEndian.Uint32(sum)
	return have == want
}
