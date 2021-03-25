package consensus

import (
	"fmt"

	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/io"
	npayload "github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

type (
	messageType byte

	message struct {
		Type           messageType
		BlockIndex     uint32
		ValidatorIndex byte
		ViewNumber     byte

		payload io.Serializable
		// stateRootEnabled specifies if state root is exchanged during consensus.
		stateRootEnabled bool
	}

	// Payload is a type for consensus-related messages.
	Payload struct {
		npayload.Extensible
		message
		network netmode.Magic
	}
)

const (
	changeViewType      messageType = 0x00
	prepareRequestType  messageType = 0x20
	prepareResponseType messageType = 0x21
	commitType          messageType = 0x30
	recoveryRequestType messageType = 0x40
	recoveryMessageType messageType = 0x41

	payloadGasLimit = 2000000 // 0.02 GAS
)

// ViewNumber implements payload.ConsensusPayload interface.
func (p Payload) ViewNumber() byte {
	return p.message.ViewNumber
}

// SetViewNumber implements payload.ConsensusPayload interface.
func (p *Payload) SetViewNumber(view byte) {
	p.message.ViewNumber = view
}

// Type implements payload.ConsensusPayload interface.
func (p Payload) Type() payload.MessageType {
	return payload.MessageType(p.message.Type)
}

// SetType implements payload.ConsensusPayload interface.
func (p *Payload) SetType(t payload.MessageType) {
	p.message.Type = messageType(t)
}

// Payload implements payload.ConsensusPayload interface.
func (p Payload) Payload() interface{} {
	return p.payload
}

// SetPayload implements payload.ConsensusPayload interface.
func (p *Payload) SetPayload(pl interface{}) {
	p.payload = pl.(io.Serializable)
}

// GetChangeView implements payload.ConsensusPayload interface.
func (p Payload) GetChangeView() payload.ChangeView { return p.payload.(payload.ChangeView) }

// GetPrepareRequest implements payload.ConsensusPayload interface.
func (p Payload) GetPrepareRequest() payload.PrepareRequest {
	return p.payload.(payload.PrepareRequest)
}

// GetPrepareResponse implements payload.ConsensusPayload interface.
func (p Payload) GetPrepareResponse() payload.PrepareResponse {
	return p.payload.(payload.PrepareResponse)
}

// GetCommit implements payload.ConsensusPayload interface.
func (p Payload) GetCommit() payload.Commit { return p.payload.(payload.Commit) }

// GetRecoveryRequest implements payload.ConsensusPayload interface.
func (p Payload) GetRecoveryRequest() payload.RecoveryRequest {
	return p.payload.(payload.RecoveryRequest)
}

// GetRecoveryMessage implements payload.ConsensusPayload interface.
func (p Payload) GetRecoveryMessage() payload.RecoveryMessage {
	return p.payload.(payload.RecoveryMessage)
}

// ValidatorIndex implements payload.ConsensusPayload interface.
func (p Payload) ValidatorIndex() uint16 {
	return uint16(p.message.ValidatorIndex)
}

// SetValidatorIndex implements payload.ConsensusPayload interface.
func (p *Payload) SetValidatorIndex(i uint16) {
	p.message.ValidatorIndex = byte(i)
}

// Height implements payload.ConsensusPayload interface.
func (p Payload) Height() uint32 {
	return p.message.BlockIndex
}

// SetHeight implements payload.ConsensusPayload interface.
func (p *Payload) SetHeight(h uint32) {
	p.message.BlockIndex = h
}

// EncodeBinary implements io.Serializable interface.
func (p *Payload) EncodeBinary(w *io.BinWriter) {
	p.encodeData()
	p.Extensible.EncodeBinary(w)
}

// Sign signs payload using the private key.
// It also sets corresponding verification and invocation scripts.
func (p *Payload) Sign(key *privateKey) error {
	p.encodeData()
	sig := key.PrivateKey.SignHashable(uint32(p.network), &p.Extensible)

	buf := io.NewBufBinWriter()
	emit.Bytes(buf.BinWriter, sig)
	p.Witness.InvocationScript = buf.Bytes()
	p.Witness.VerificationScript = key.PublicKey().GetVerificationScript()

	return nil
}

// Hash implements payload.ConsensusPayload interface.
func (p *Payload) Hash() util.Uint256 {
	if p.Extensible.Data == nil {
		p.encodeData()
	}
	return p.Extensible.Hash()
}

// DecodeBinary implements io.Serializable interface.
func (p *Payload) DecodeBinary(r *io.BinReader) {
	p.Extensible.DecodeBinary(r)
	p.decodeData()
}

// EncodeBinary implements io.Serializable interface.
func (m *message) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(m.Type))
	w.WriteU32LE(m.BlockIndex)
	w.WriteB(m.ValidatorIndex)
	w.WriteB(m.ViewNumber)
	m.payload.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable interface.
func (m *message) DecodeBinary(r *io.BinReader) {
	m.Type = messageType(r.ReadB())
	m.BlockIndex = r.ReadU32LE()
	m.ValidatorIndex = r.ReadB()
	m.ViewNumber = r.ReadB()

	switch m.Type {
	case changeViewType:
		cv := new(changeView)
		// newViewNumber is not marshaled
		cv.newViewNumber = m.ViewNumber + 1
		m.payload = cv
	case prepareRequestType:
		r := new(prepareRequest)
		if m.stateRootEnabled {
			r.stateRootEnabled = true
		}
		m.payload = r
	case prepareResponseType:
		m.payload = new(prepareResponse)
	case commitType:
		m.payload = new(commit)
	case recoveryRequestType:
		m.payload = new(recoveryRequest)
	case recoveryMessageType:
		r := new(recoveryMessage)
		if m.stateRootEnabled {
			r.stateRootEnabled = true
		}
		m.payload = r
	default:
		r.Err = fmt.Errorf("invalid type: 0x%02x", byte(m.Type))
		return
	}
	m.payload.DecodeBinary(r)
}

// String implements fmt.Stringer interface.
func (t messageType) String() string {
	switch t {
	case changeViewType:
		return "ChangeView"
	case prepareRequestType:
		return "PrepareRequest"
	case prepareResponseType:
		return "PrepareResponse"
	case commitType:
		return "Commit"
	case recoveryRequestType:
		return "RecoveryRequest"
	case recoveryMessageType:
		return "RecoveryMessage"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02x)", byte(t))
	}
}

func (p *Payload) encodeData() {
	if p.Extensible.Data == nil {
		p.Extensible.ValidBlockStart = 0
		p.Extensible.ValidBlockEnd = p.BlockIndex
		bw := io.NewBufBinWriter()
		p.message.EncodeBinary(bw.BinWriter)
		p.Extensible.Data = bw.Bytes()
	}
}

// decode data of payload into it's message
func (p *Payload) decodeData() error {
	br := io.NewBinReaderFromBuf(p.Extensible.Data)
	p.message.DecodeBinary(br)
	if br.Err != nil {
		return fmt.Errorf("can't decode message: %w", br.Err)
	}
	return nil
}
