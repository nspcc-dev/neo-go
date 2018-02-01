package network

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core"
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

// NetMode type that is compatible with netModes below.
type NetMode uint32

// String implements the stringer interface.
func (n NetMode) String() string {
	switch n {
	case ModeDevNet:
		return "devnet"
	case ModeTestNet:
		return "testnet"
	case ModeMainNet:
		return "mainnet"
	default:
		return ""
	}
}

// Values used for the magic field, according to the docs.
const (
	ModeMainNet NetMode = 0x00746e41 // 7630401
	ModeTestNet         = 0x74746e41 // 1953787457
	ModeDevNet          = 56753      // docker privnet
)

// Message is the complete message send between nodes.
type Message struct {
	Magic NetMode
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

type commandType string

// valid commands used to send between nodes.
const (
	cmdVersion    commandType = "version"
	cmdVerack                 = "verack"
	cmdGetAddr                = "getaddr"
	cmdAddr                   = "addr"
	cmdGetHeaders             = "getheaders"
	cmdHeaders                = "headers"
	cmdGetBlocks              = "getblocks"
	cmdInv                    = "inv"
	cmdGetData                = "getdata"
	cmdBlock                  = "block"
	cmdTX                     = "tx"
	cmdConsensus              = "consensus"
)

func newMessage(magic NetMode, cmd commandType, p payload.Payload) *Message {
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

// Converts the 12 byte command slice to a commandType.
func (m *Message) commandType() commandType {
	cmd := cmdByteArrayToString(m.Command)
	switch cmd {
	case "version":
		return cmdVersion
	case "verack":
		return cmdVerack
	case "getaddr":
		return cmdGetAddr
	case "addr":
		return cmdAddr
	case "getheaders":
		return cmdGetHeaders
	case "header":
		return cmdHeaders
	case "getblocks":
		return cmdGetBlocks
	case "inv":
		return cmdInv
	case "getdata":
		return cmdGetData
	case "block":
		return cmdBlock
	case "tx":
		return cmdTX
	case "consensus":
		return cmdConsensus
	default:
		return ""
	}
}

// decode a Message from the given reader.
func (m *Message) decode(r io.Reader) error {
	binary.Read(r, binary.LittleEndian, &m.Magic)
	binary.Read(r, binary.LittleEndian, &m.Command)
	binary.Read(r, binary.LittleEndian, &m.Length)
	binary.Read(r, binary.LittleEndian, &m.Checksum)

	// return if their is no payload.
	if m.Length == 0 {
		return nil
	}

	return m.decodePayload(r)
}

func (m *Message) decodePayload(r io.Reader) error {
	buf := make([]byte, m.Length)
	n, err := r.Read(buf)
	if err != nil {
		return err
	}

	if uint32(n) != m.Length {
		return fmt.Errorf("expected to have read exactly %d bytes got %d", m.Length, n)
	}

	// Compare the checksum of the payload.
	if !compareChecksum(m.Checksum, buf) {
		return errChecksumMismatch
	}

	r = bytes.NewReader(buf)
	var p payload.Payload
	switch m.commandType() {
	case cmdVersion:
		p = &payload.Version{}
		if err := p.DecodeBinary(r); err != nil {
			return err
		}
	case cmdInv:
		p = &payload.Inventory{}
		if err := p.DecodeBinary(r); err != nil {
			return err
		}
	case cmdAddr:
		p = &payload.AddressList{}
		if err := p.DecodeBinary(r); err != nil {
			return err
		}
	case cmdBlock:
		p = &core.Block{}
		if err := p.DecodeBinary(r); err != nil {
			return err
		}
	}

	m.Payload = p

	return nil
}

// encode a Message to any given io.Writer.
func (m *Message) encode(w io.Writer) error {
	binary.Write(w, binary.LittleEndian, m.Magic)
	binary.Write(w, binary.LittleEndian, m.Command)
	binary.Write(w, binary.LittleEndian, m.Length)
	binary.Write(w, binary.LittleEndian, m.Checksum)

	if m.Payload != nil {
		return m.Payload.EncodeBinary(w)
	}

	return nil
}

// convert a command (string) to a byte slice filled with 0 bytes till
// size 12.
func cmdToByteArray(cmd commandType) [cmdSize]byte {
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
