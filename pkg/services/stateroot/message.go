package stateroot

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

type (
	// MessageType represents message type.
	MessageType byte

	// Message represents state-root related message.
	Message struct {
		Network netmode.Magic
		Type    MessageType
		Payload io.Serializable
	}
)

// Various message types.
const (
	VoteT MessageType = 0
	RootT MessageType = 1
)

// NewMessage creates new message of specified type.
func NewMessage(net netmode.Magic, typ MessageType, p io.Serializable) *Message {
	return &Message{
		Network: net,
		Type:    typ,
		Payload: p,
	}
}

// EncodeBinary implements io.Serializable interface.
func (m *Message) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(m.Type))
	m.Payload.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable interface.
func (m *Message) DecodeBinary(r *io.BinReader) {
	switch m.Type = MessageType(r.ReadB()); m.Type {
	case VoteT:
		m.Payload = new(Vote)
	case RootT:
		m.Payload = &state.MPTRoot{Network: m.Network}
	default:
		r.Err = fmt.Errorf("invalid type: %x", m.Type)
		return
	}
	m.Payload.DecodeBinary(r)
}
