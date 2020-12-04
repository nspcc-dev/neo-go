package network

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/consensus"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
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

	// Length of the payload.
	Length uint32

	// Checksum is the first 4 bytes of the value that two times SHA256
	// hash of the payload.
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
	CMDGetRoots    CommandType = "getroots"
	CMDHeaders     CommandType = "headers"
	CMDInv         CommandType = "inv"
	CMDMempool     CommandType = "mempool"
	CMDMerkleBlock CommandType = "merkleblock"
	CMDPing        CommandType = "ping"
	CMDPong        CommandType = "pong"
	CMDRoots       CommandType = "roots"
	CMDStateRoot   CommandType = "stateroot"
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
		buf := io.NewBufBinWriter()
		p.EncodeBinary(buf.BinWriter)
		if buf.Err != nil {
			panic(buf.Err)
		}
		b := buf.Bytes()
		size = uint32(len(b))
		checksum = hash.Checksum(b)
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
	case "getroots":
		return CMDGetRoots
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
	case "roots":
		return CMDRoots
	case "stateroot":
		return CMDStateRoot
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

// Decode decodes a Message from the given reader.
func (m *Message) Decode(br *io.BinReader) error {
	m.Magic = config.NetMode(br.ReadU32LE())
	br.ReadBytes(m.Command[:])
	m.Length = br.ReadU32LE()
	m.Checksum = br.ReadU32LE()
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
	// Compare the checksum of the payload.
	if !compareChecksum(m.Checksum, buf) {
		return errChecksumMismatch
	}

	r := io.NewBinReaderFromBuf(buf)
	var p payload.Payload
	switch m.CommandType() {
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
	case CMDGetRoots:
		p = &payload.GetStateRoots{}
	case CMDHeaders:
		p = &payload.Headers{}
	case CMDTX:
		p = &transaction.Transaction{}
	case CMDMerkleBlock:
		p = &payload.MerkleBlock{}
	case CMDPing, CMDPong:
		p = &payload.Ping{}
	case CMDRoots:
		p = &payload.StateRoots{}
	case CMDStateRoot:
		p = &state.MPTRoot{}
	default:
		return fmt.Errorf("can't decode command %s", cmdByteArrayToString(m.Command))
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
	br.WriteBytes(m.Command[:])
	br.WriteU32LE(m.Length)
	br.WriteU32LE(m.Checksum)
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
	return w.Bytes(), nil
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
