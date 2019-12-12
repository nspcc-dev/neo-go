package consensus

import (
	"crypto/sha256"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/pkg/errors"
)

type (
	messageType byte

	message struct {
		Type       messageType
		ViewNumber byte

		payload io.Serializable
	}

	// Payload is a type for consensus-related messages.
	Payload struct {
		message

		version        uint32
		validatorIndex uint16
		prevHash       util.Uint256
		height         uint32
		timestamp      uint32

		Witness transaction.Witness
	}
)

const (
	changeViewType      messageType = 0x00
	prepareRequestType  messageType = 0x20
	prepareResponseType messageType = 0x21
	commitType          messageType = 0x30
	recoveryRequestType messageType = 0x40
	recoveryMessageType messageType = 0x41
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

// MarshalUnsigned implements payload.ConsensusPayload interface.
func (p Payload) MarshalUnsigned() []byte {
	w := io.NewBufBinWriter()
	p.EncodeBinaryUnsigned(w.BinWriter)

	return w.Bytes()
}

// UnmarshalUnsigned implements payload.ConsensusPayload interface.
func (p *Payload) UnmarshalUnsigned(data []byte) error {
	r := io.NewBinReaderFromBuf(data)
	p.DecodeBinaryUnsigned(r)

	return r.Err
}

// Version implements payload.ConsensusPayload interface.
func (p Payload) Version() uint32 {
	return p.version
}

// SetVersion implements payload.ConsensusPayload interface.
func (p *Payload) SetVersion(v uint32) {
	p.version = v
}

// ValidatorIndex implements payload.ConsensusPayload interface.
func (p Payload) ValidatorIndex() uint16 {
	return p.validatorIndex
}

// SetValidatorIndex implements payload.ConsensusPayload interface.
func (p *Payload) SetValidatorIndex(i uint16) {
	p.validatorIndex = i
}

// PrevHash implements payload.ConsensusPayload interface.
func (p Payload) PrevHash() util.Uint256 {
	return p.prevHash
}

// SetPrevHash implements payload.ConsensusPayload interface.
func (p *Payload) SetPrevHash(h util.Uint256) {
	p.prevHash = h
}

// Height implements payload.ConsensusPayload interface.
func (p Payload) Height() uint32 {
	return p.height
}

// SetHeight implements payload.ConsensusPayload interface.
func (p *Payload) SetHeight(h uint32) {
	p.height = h
}

// EncodeBinaryUnsigned writes payload to w excluding signature.
func (p *Payload) EncodeBinaryUnsigned(w *io.BinWriter) {
	w.WriteU32LE(p.version)
	w.WriteBytes(p.prevHash[:])
	w.WriteU32LE(p.height)
	w.WriteU16LE(p.validatorIndex)
	w.WriteU32LE(p.timestamp)

	ww := io.NewBufBinWriter()
	p.message.EncodeBinary(ww.BinWriter)
	w.WriteVarBytes(ww.Bytes())
}

// EncodeBinary implements io.Serializable interface.
func (p *Payload) EncodeBinary(w *io.BinWriter) {
	p.EncodeBinaryUnsigned(w)

	w.WriteB(1)
	p.Witness.EncodeBinary(w)
}

// Sign signs payload using the private key.
// It also sets corresponding verification and invocation scripts.
func (p *Payload) Sign(key *privateKey) error {
	sig, err := key.Sign(p.MarshalUnsigned())
	if err != nil {
		return err
	}

	p.Witness.InvocationScript = append([]byte{byte(opcode.PUSHBYTES64)}, sig...)
	p.Witness.VerificationScript = key.PublicKey().GetVerificationScript()

	return nil
}

// Verify verifies payload using provided Witness.
func (p *Payload) Verify() bool {
	h := sha256.Sum256(p.MarshalUnsigned())
	v := vm.New()
	v.SetCheckedHash(h[:])
	v.Load(append(p.Witness.InvocationScript, p.Witness.VerificationScript...))
	if err := v.Run(); err != nil || v.Estack().Len() == 0 {
		return false
	}

	result, err := v.Estack().Top().TryBool()

	return err == nil && result
}

// DecodeBinaryUnsigned reads payload from w excluding signature.
func (p *Payload) DecodeBinaryUnsigned(r *io.BinReader) {
	p.version = r.ReadU32LE()
	r.ReadBytes(p.prevHash[:])
	p.height = r.ReadU32LE()
	p.validatorIndex = r.ReadU16LE()
	p.timestamp = r.ReadU32LE()

	data := r.ReadVarBytes()
	if r.Err != nil {
		return
	}

	rr := io.NewBinReaderFromBuf(data)
	p.message.DecodeBinary(rr)
	r.Err = rr.Err
}

// Hash implements payload.ConsensusPayload interface.
func (p *Payload) Hash() util.Uint256 {
	w := io.NewBufBinWriter()
	p.EncodeBinaryUnsigned(w.BinWriter)

	return hash.DoubleSha256(w.Bytes())
}

// DecodeBinary implements io.Serializable interface.
func (p *Payload) DecodeBinary(r *io.BinReader) {
	p.DecodeBinaryUnsigned(r)
	if r.Err != nil {
		return
	}

	var b = r.ReadB()
	if b != 1 {
		r.Err = errors.New("invalid format")
		return
	}

	p.Witness.DecodeBinary(r)
}

// EncodeBinary implements io.Serializable interface.
func (m *message) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes([]byte{byte(m.Type)})
	w.WriteB(m.ViewNumber)
	m.payload.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable interface.
func (m *message) DecodeBinary(r *io.BinReader) {
	m.Type = messageType(r.ReadB())
	m.ViewNumber = r.ReadB()

	switch m.Type {
	case changeViewType:
		cv := new(changeView)
		// newViewNumber is not marshaled
		cv.newViewNumber = m.ViewNumber + 1
		m.payload = cv
	case prepareRequestType:
		m.payload = new(prepareRequest)
	case prepareResponseType:
		m.payload = new(prepareResponse)
	case commitType:
		m.payload = new(commit)
	case recoveryRequestType:
		m.payload = new(recoveryRequest)
	case recoveryMessageType:
		m.payload = new(recoveryMessage)
	default:
		r.Err = errors.Errorf("invalid type: 0x%02x", byte(m.Type))
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
