package network

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/anthdm/neo-go/pkg/network/payload"
)

const (
	// The minimum size of a valid message.
	minMessageSize = 24
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
//
//	Size	Field		DataType		Description
// 	------------------------------------------------------
//	4		Magic		uint32			Protocol ID
//	12		Command		char[12]		Command
//	4		length		uint32			Length of payload
//	4		Checksum	uint32			Checksum
//	length	Payload		uint8[length]	Content of message
type Message struct {
	Magic NetMode
	// Command is utf8 code, of which the length is 12 bytes,
	// the extra part is filled with 0.
	Command []byte
	// Length of the payload
	Length uint32
	// Checksum is the first 4 bytes of the value that two times SHA256
	// hash of the payload
	Checksum uint32
	// Payload send with the message.
	Payload payload.Payloader
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
)

func newMessage(magic NetMode, cmd commandType, p payload.Payloader) *Message {
	var size uint32
	if p != nil {
		size = p.Size()
	}
	return &Message{
		Magic:   magic,
		Command: cmdToByteSlice(cmd),
		Length:  size,
		Payload: p,
	}
}

// Converts the 12 byte command slice to a commandType.
func (m *Message) commandType() commandType {
	cmd := string(bytes.TrimRight(m.Command, "\x00"))
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
	default:
		return ""
	}
}

// decode a Message from the given reader.
func (m *Message) decode(r io.Reader) error {
	// 24 bytes for the fixed sized fields.
	buf := make([]byte, minMessageSize)
	if _, err := r.Read(buf); err != nil {
		return err
	}

	m.Magic = NetMode(binary.LittleEndian.Uint32(buf[0:4]))
	m.Command = buf[4:16]
	m.Length = binary.LittleEndian.Uint32(buf[16:20])
	m.Checksum = binary.LittleEndian.Uint32(buf[20:24])

	// if their is no payload.
	if m.Length == 0 || !needPayloadDecode(m.commandType()) {
		return nil
	}

	return m.decodePayload(r)
}

func (m *Message) decodePayload(r io.Reader) error {
	// write to a buffer what we read to calculate the checksum.
	buffer := new(bytes.Buffer)
	tr := io.TeeReader(r, buffer)
	var p payload.Payloader

	switch m.commandType() {
	case cmdVersion:
		p = &payload.Version{}
		if err := p.Decode(tr); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown command to decode: %s", m.commandType())
	}

	// Compare the checksum of the payload.
	if !compareChecksum(m.Checksum, buffer.Bytes()) {
		return errors.New("checksum mismatch error")
	}

	m.Payload = p

	return nil
}

// encode a Message to any given io.Writer.
func (m *Message) encode(w io.Writer) error {
	buf := make([]byte, minMessageSize)
	pbuf := new(bytes.Buffer)

	// if there is a payload fill its allocated buffer.
	var checksum []byte
	if m.Payload != nil {
		if err := m.Payload.Encode(pbuf); err != nil {
			return err
		}
		checksum = sumSHA256(sumSHA256(pbuf.Bytes()))[:4]
	} else {
		checksum = sumSHA256(sumSHA256([]byte{}))[:4]
	}

	m.Checksum = binary.LittleEndian.Uint32(checksum)

	// fill the message buffer
	binary.LittleEndian.PutUint32(buf[0:4], uint32(m.Magic))
	copy(buf[4:16], m.Command)
	binary.LittleEndian.PutUint32(buf[16:20], m.Length)
	binary.LittleEndian.PutUint32(buf[20:24], m.Checksum)

	// write the message
	n, err := w.Write(buf)
	if err != nil {
		return err
	}

	// we need to have at least writen exactly minMessageSize bytes.
	if n != minMessageSize {
		return errors.New("long/short read error when encoding message")
	}

	// write the payload if there was any
	if pbuf.Len() > 0 {
		n, err = w.Write(pbuf.Bytes())
		if err != nil {
			return err
		}

		if uint32(n) != m.Payload.Size() {
			return errors.New("long/short read error when encoding payload")
		}
	}

	return nil
}

// convert a command (string) to a byte slice filled with 0 bytes till
// size 12.
func cmdToByteSlice(cmd commandType) []byte {
	cmdLen := len(cmd)
	if cmdLen > 12 {
		panic("exceeded command max length of size 12")
	}

	// The command can have max 12 bytes, rest is filled with 0.
	b := []byte(cmd)
	for i := 0; i < 12-cmdLen; i++ {
		b = append(b, byte('\x00'))
	}

	return b
}

func needPayloadDecode(cmd commandType) bool {
	return cmd != cmdVerack && cmd != cmdGetAddr
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
