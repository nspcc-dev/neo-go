package network

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
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
	CMDVersion    CommandType = "version"
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
		checksum = sumSHA256(sumSHA256(buf.Bytes()))
	} else {
		checksum = sumSHA256(sumSHA256([]byte{}))
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
	case "version":
		return CMDVersion
	case "verack":
		return CMDVerack
	case "getaddr":
		return CMDGetAddr
	case "addr":
		return CMDAddr
	case "getheaders":
		return CMDGetHeaders
	case "headers":
		return CMDHeaders
	case "getblocks":
		return CMDGetBlocks
	case "inv":
		return CMDInv
	case "getdata":
		return CMDGetData
	case "block":
		return CMDBlock
	case "tx":
		return CMDTX
	case "consensus":
		return CMDConsensus
	default:
		return CMDUnknown
	}
}

// Decode a Message from the given reader.
func (m *Message) Decode(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &m.Magic); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &m.Command); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &m.Length); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &m.Checksum); err != nil {
		return err
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
	}

	m.Payload = p

	return nil
}

// Encode a Message to any given io.Writer.
func (m *Message) Encode(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, m.Magic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, m.Command); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, m.Length); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, m.Checksum); err != nil {
		return err
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
	buf := []byte{}
	for i := 0; i < cmdSize; i++ {
		if cmd[i] != 0 {
			buf = append(buf, cmd[i])
		}
	}
	return string(buf)
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
