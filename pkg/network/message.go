package network

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/consensus"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
)

//go:generate stringer -type=CommandType

// Message is the complete message send between nodes.
type Message struct {
	// NetMode of the node that sends this message.
	Magic config.NetMode

	// Command is byte command code.
	Command CommandType

	// Length of the payload.
	Length uint32

	// Payload send with the message.
	Payload payload.Payload
}

// CommandType represents the type of a message command.
type CommandType byte

// Valid protocol commands used to send between nodes.
const (
	// handshaking
	CMDVersion CommandType = 0x00
	CMDVerack  CommandType = 0x01

	// connectivity
	CMDGetAddr CommandType = 0x10
	CMDAddr    CommandType = 0x11
	CMDPing    CommandType = 0x18
	CMDPong    CommandType = 0x19

	// synchronization
	CMDGetHeaders CommandType = 0x20
	CMDHeaders    CommandType = 0x21
	CMDGetBlocks  CommandType = 0x24
	CMDMempool    CommandType = 0x25
	CMDInv        CommandType = 0x27
	CMDGetData    CommandType = 0x28
	CMDUnknown    CommandType = 0x2a
	CMDTX         CommandType = 0x2b
	CMDBlock      CommandType = 0x2c
	CMDConsensus  CommandType = 0x2d
	CMDReject     CommandType = 0x2f

	// SPV protocol
	CMDFilterLoad  CommandType = 0x30
	CMDFilterAdd   CommandType = 0x31
	CMDFilterClear CommandType = 0x32
	CMDMerkleBlock CommandType = 0x38

	// others
	CMDAlert CommandType = 0x40
)

// NewMessage returns a new message with the given payload.
func NewMessage(magic config.NetMode, cmd CommandType, p payload.Payload) *Message {
	var (
		size uint32
	)

	if p != nil {
		buf := io.NewBufBinWriter()
		p.EncodeBinary(buf.BinWriter)
		if buf.Err != nil {
			panic(buf.Err)
		}
		b := buf.Bytes()
		size = uint32(len(b))
	}

	return &Message{
		Magic:   magic,
		Command: cmd,
		Length:  size,
		Payload: p,
	}
}

// Decode decodes a Message from the given reader.
func (m *Message) Decode(br *io.BinReader) error {
	m.Magic = config.NetMode(br.ReadU32LE())
	m.Command = CommandType(br.ReadB())
	m.Length = br.ReadU32LE()
	if br.Err != nil {
		return br.Err
	}
	// return if their is no payload.
	if m.Length == 0 {
		return nil
	}
	return m.decodePayload(br)
}

func (m *Message) decodePayload(br *io.BinReader) error {
	buf := make([]byte, m.Length)
	br.ReadBytes(buf)
	if br.Err != nil {
		return br.Err
	}

	r := io.NewBinReaderFromBuf(buf)
	var p payload.Payload
	switch m.Command {
	case CMDVersion:
		p = &payload.Version{}
	case CMDInv, CMDGetData:
		p = &payload.Inventory{}
	case CMDAddr:
		p = &payload.AddressList{}
	case CMDBlock:
		p = &block.Block{}
	case CMDConsensus:
		p = &consensus.Payload{}
	case CMDGetBlocks:
		fallthrough
	case CMDGetHeaders:
		p = &payload.GetBlocks{}
	case CMDHeaders:
		p = &payload.Headers{}
	case CMDTX:
		p = &transaction.Transaction{}
	case CMDMerkleBlock:
		p = &payload.MerkleBlock{}
	case CMDPing, CMDPong:
		p = &payload.Ping{}
	default:
		return fmt.Errorf("can't decode command %s", m.Command.String())
	}
	p.DecodeBinary(r)
	if r.Err == nil || r.Err == payload.ErrTooManyHeaders {
		m.Payload = p
	}

	return r.Err
}

// Encode encodes a Message to any given BinWriter.
func (m *Message) Encode(br *io.BinWriter) error {
	br.WriteU32LE(uint32(m.Magic))
	br.WriteB(byte(m.Command))
	br.WriteU32LE(m.Length)
	if m.Payload != nil {
		m.Payload.EncodeBinary(br)

	}
	if br.Err != nil {
		return br.Err
	}
	return nil
}

// Bytes serializes a Message into the new allocated buffer and returns it.
func (m *Message) Bytes() ([]byte, error) {
	w := io.NewBufBinWriter()
	if err := m.Encode(w.BinWriter); err != nil {
		return nil, err
	}
	if w.Err != nil {
		return nil, w.Err
	}
	return w.Bytes(), nil
}
