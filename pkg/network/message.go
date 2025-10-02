package network

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
)

//go:generate stringer -type=CommandType -output=message_string.go

// CompressionMinSize is the lower bound to apply compression.
const CompressionMinSize = 1024

// Message is a complete message sent between nodes.
type Message struct {
	// Flags that represents whether a message is compressed.
	// 0 for None, 1 for Compressed.
	Flags MessageFlag
	// Command is a byte command code.
	Command CommandType

	// Payload send with the message.
	Payload payload.Payload

	// Compressed message payload.
	compressedPayload []byte
}

// MessageFlag represents compression level of a message payload.
type MessageFlag byte

// Possible message flags.
const (
	Compressed MessageFlag = 1 << iota
	None       MessageFlag = 0
)

// CommandType represents the type of a message command.
type CommandType byte

// Valid protocol commands used to send between nodes.
const (
	// Handshaking.
	CMDVersion CommandType = 0x00
	CMDVerack  CommandType = 0x01

	// Connectivity.
	CMDGetAddr CommandType = 0x10
	CMDAddr    CommandType = 0x11
	CMDPing    CommandType = 0x18
	CMDPong    CommandType = 0x19

	// Synchronization.
	CMDGetHeaders       CommandType = 0x20
	CMDHeaders          CommandType = 0x21
	CMDGetBlocks        CommandType = 0x24
	CMDMempool          CommandType = 0x25
	CMDInv              CommandType = 0x27
	CMDGetData          CommandType = 0x28
	CMDGetBlockByIndex  CommandType = 0x29
	CMDNotFound         CommandType = 0x2a
	CMDTX                           = CommandType(payload.TXType)
	CMDBlock                        = CommandType(payload.BlockType)
	CMDExtensible                   = CommandType(payload.ExtensibleType)
	CMDP2PNotaryRequest             = CommandType(payload.P2PNotaryRequestType)
	CMDGetMPTData       CommandType = 0x51 // 0x5.. commands are used for extensions (P2PNotary, state exchange cmds)
	CMDMPTData          CommandType = 0x52
	CMDReject           CommandType = 0x2f

	// SPV protocol.
	CMDFilterLoad  CommandType = 0x30
	CMDFilterAdd   CommandType = 0x31
	CMDFilterClear CommandType = 0x32
	CMDMerkleBlock CommandType = 0x38

	// Others.
	CMDAlert CommandType = 0x40
)

// NewMessage returns a new message with the given payload.
func NewMessage(cmd CommandType, p payload.Payload) *Message {
	return &Message{
		Command: cmd,
		Payload: p,
		Flags:   None,
	}
}

// Decode decodes a Message from the given reader.
func (m *Message) Decode(br *io.BinReader) error {
	m.Flags = MessageFlag(br.ReadB())
	m.Command = CommandType(br.ReadB())
	l := br.ReadVarUint()
	if br.Err != nil {
		return br.Err
	}
	// check the length first in order not to allocate memory
	// for an empty compressed payload
	if l == 0 {
		switch m.Command {
		case CMDFilterClear, CMDGetAddr, CMDMempool, CMDVerack:
			m.Payload = payload.NewNullPayload()
		default:
			return fmt.Errorf("unexpected empty payload: %s", m.Command)
		}
		return nil
	}
	if l > payload.MaxSize {
		return errors.New("invalid payload size")
	}
	m.compressedPayload = make([]byte, l)
	br.ReadBytes(m.compressedPayload)
	if br.Err != nil {
		return br.Err
	}
	return m.decodePayload()
}

func (m *Message) decodePayload() error {
	buf := m.compressedPayload
	// try decompression
	if m.Flags&Compressed != 0 {
		d, err := decompress(m.compressedPayload)
		if err != nil {
			return err
		}
		buf = d
	}

	var p payload.Payload
	switch m.Command {
	case CMDVersion:
		p = &payload.Version{}
	case CMDInv, CMDGetData:
		p = &payload.Inventory{}
	case CMDGetMPTData:
		p = &payload.MPTInventory{}
	case CMDMPTData:
		p = &payload.MPTData{}
	case CMDAddr:
		p = &payload.AddressList{}
	case CMDBlock:
		p = &block.Block{}
	case CMDExtensible:
		p = payload.NewExtensible()
	case CMDP2PNotaryRequest:
		p = &payload.P2PNotaryRequest{}
	case CMDGetBlocks:
		p = &payload.GetBlocks{}
	case CMDGetHeaders:
		fallthrough
	case CMDGetBlockByIndex:
		p = &payload.GetBlockByIndex{}
	case CMDHeaders:
		p = &payload.Headers{}
	case CMDTX:
		p, err := transaction.NewTransactionFromBytes(buf)
		if err != nil {
			return err
		}
		m.Payload = p
		return nil
	case CMDMerkleBlock:
		p = &payload.MerkleBlock{}
	case CMDPing, CMDPong:
		p = &payload.Ping{}
	case CMDNotFound:
		p = &payload.Inventory{}
	default:
		return fmt.Errorf("can't decode command %s", m.Command.String())
	}
	r := io.NewBinReaderFromBuf(buf)
	p.DecodeBinary(r)
	if r.Err == nil || errors.Is(r.Err, payload.ErrTooManyHeaders) {
		m.Payload = p
	}

	return r.Err
}

// Encode encodes a Message to any given BinWriter.
func (m *Message) Encode(br *io.BinWriter) error {
	return m.EncodeCompressed(br, true)
}

// EncodeCompressed encodes a Message to any given BinWriter with possible
// compression for large payloads if allowCompression is set.
func (m *Message) EncodeCompressed(br *io.BinWriter, allowCompression bool) error {
	if err := m.tryCompressPayload(allowCompression); err != nil {
		return err
	}
	growSize := 2 + 1 // header + empty payload
	if m.compressedPayload != nil {
		growSize += 8 + len(m.compressedPayload) // varint + byte-slice
	}
	br.Grow(growSize)
	br.WriteB(byte(m.Flags))
	br.WriteB(byte(m.Command))
	if m.compressedPayload != nil {
		br.WriteVarBytes(m.compressedPayload)
	} else {
		br.WriteB(0)
	}
	return br.Err
}

// Bytes serializes a Message into the new allocated buffer and returns it.
func (m *Message) Bytes() ([]byte, error) {
	return m.BytesCompressed(true)
}

// BytesCompressed serializes a Message into the new allocated buffer with possible
// compression for large payloads if allowCompression is set and returns serialized
// Message.
func (m *Message) BytesCompressed(allowCompression bool) ([]byte, error) {
	w := io.NewBufBinWriter()
	if err := m.EncodeCompressed(w.BinWriter, allowCompression); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// tryCompressPayload sets the message's compressed payload to a serialized
// payload and compresses it in case its size exceeds CompressionMinSize.
func (m *Message) tryCompressPayload(enableCompression bool) error {
	if m.Payload == nil {
		return nil
	}
	buf := io.NewBufBinWriter()
	m.Payload.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	compressedPayload := buf.Bytes()
	if m.Flags&Compressed == 0 && enableCompression {
		switch m.Payload.(type) {
		case *payload.Headers, *payload.MerkleBlock, payload.NullPayload,
			*payload.Inventory, *payload.MPTInventory:
			break
		default:
			size := len(compressedPayload)
			// try compression
			if size > CompressionMinSize {
				c, err := compress(compressedPayload)
				if err == nil {
					compressedPayload = c
					m.Flags |= Compressed
				} else {
					return err
				}
			}
		}
	}
	m.compressedPayload = compressedPayload
	return nil
}
