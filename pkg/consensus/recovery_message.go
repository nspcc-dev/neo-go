package consensus

import (
	"errors"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	npayload "github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	// recoveryMessage represents dBFT Recovery message.
	recoveryMessage struct {
		preparationHash     *util.Uint256
		preparationPayloads []*preparationCompact
		commitPayloads      []*commitCompact
		changeViewPayloads  []*changeViewCompact
		stateRootEnabled    bool
		prepareRequest      *message
	}

	changeViewCompact struct {
		ValidatorIndex     uint8
		OriginalViewNumber byte
		Timestamp          uint64
		InvocationScript   []byte
	}

	commitCompact struct {
		ViewNumber       byte
		ValidatorIndex   uint8
		Signature        [signatureSize]byte
		InvocationScript []byte
	}

	preparationCompact struct {
		ValidatorIndex   uint8
		InvocationScript []byte
	}
)

var _ dbft.RecoveryMessage[util.Uint256] = (*recoveryMessage)(nil)

// DecodeBinary implements the io.Serializable interface.
func (m *recoveryMessage) DecodeBinary(r *io.BinReader) {
	r.ReadArray(&m.changeViewPayloads)

	var hasReq = r.ReadBool()
	if hasReq {
		m.prepareRequest = &message{stateRootEnabled: m.stateRootEnabled}
		m.prepareRequest.DecodeBinary(r)
		if r.Err == nil && m.prepareRequest.Type != prepareRequestType {
			r.Err = errors.New("recovery message PrepareRequest has wrong type")
			return
		}
	} else {
		l := r.ReadVarUint()
		if l != 0 {
			if l == util.Uint256Size {
				m.preparationHash = new(util.Uint256)
				r.ReadBytes(m.preparationHash[:])
			} else {
				r.Err = errors.New("invalid data")
			}
		} else {
			m.preparationHash = nil
		}
	}

	r.ReadArray(&m.preparationPayloads)
	r.ReadArray(&m.commitPayloads)
}

// EncodeBinary implements the io.Serializable interface.
func (m *recoveryMessage) EncodeBinary(w *io.BinWriter) {
	w.WriteArray(m.changeViewPayloads)

	hasReq := m.prepareRequest != nil
	w.WriteBool(hasReq)
	if hasReq {
		m.prepareRequest.EncodeBinary(w)
	} else {
		if m.preparationHash == nil {
			w.WriteVarUint(0)
		} else {
			w.WriteVarUint(util.Uint256Size)
			w.WriteBytes(m.preparationHash[:])
		}
	}

	w.WriteArray(m.preparationPayloads)
	w.WriteArray(m.commitPayloads)
}

// DecodeBinary implements the io.Serializable interface.
func (p *changeViewCompact) DecodeBinary(r *io.BinReader) {
	p.ValidatorIndex = r.ReadB()
	p.OriginalViewNumber = r.ReadB()
	p.Timestamp = r.ReadU64LE()
	p.InvocationScript = r.ReadVarBytes(1024)
}

// EncodeBinary implements the io.Serializable interface.
func (p *changeViewCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteB(p.ValidatorIndex)
	w.WriteB(p.OriginalViewNumber)
	w.WriteU64LE(p.Timestamp)
	w.WriteVarBytes(p.InvocationScript)
}

// DecodeBinary implements the io.Serializable interface.
func (p *commitCompact) DecodeBinary(r *io.BinReader) {
	p.ViewNumber = r.ReadB()
	p.ValidatorIndex = r.ReadB()
	r.ReadBytes(p.Signature[:])
	p.InvocationScript = r.ReadVarBytes(1024)
}

// EncodeBinary implements the io.Serializable interface.
func (p *commitCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteB(p.ViewNumber)
	w.WriteB(p.ValidatorIndex)
	w.WriteBytes(p.Signature[:])
	w.WriteVarBytes(p.InvocationScript)
}

// DecodeBinary implements the io.Serializable interface.
func (p *preparationCompact) DecodeBinary(r *io.BinReader) {
	p.ValidatorIndex = r.ReadB()
	p.InvocationScript = r.ReadVarBytes(1024)
}

// EncodeBinary implements the io.Serializable interface.
func (p *preparationCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteB(p.ValidatorIndex)
	w.WriteVarBytes(p.InvocationScript)
}

// AddPayload implements the payload.RecoveryMessage interface.
func (m *recoveryMessage) AddPayload(p dbft.ConsensusPayload[util.Uint256]) {
	validator := uint8(p.ValidatorIndex())

	switch p.Type() {
	case dbft.PrepareRequestType:
		m.prepareRequest = &message{
			Type:             prepareRequestType,
			ViewNumber:       p.ViewNumber(),
			payload:          p.GetPrepareRequest().(*prepareRequest),
			stateRootEnabled: m.stateRootEnabled,
		}
		h := p.Hash()
		m.preparationHash = &h
		m.preparationPayloads = append(m.preparationPayloads, &preparationCompact{
			ValidatorIndex:   validator,
			InvocationScript: p.(*Payload).Witness.InvocationScript,
		})
	case dbft.PrepareResponseType:
		m.preparationPayloads = append(m.preparationPayloads, &preparationCompact{
			ValidatorIndex:   validator,
			InvocationScript: p.(*Payload).Witness.InvocationScript,
		})

		if m.preparationHash == nil {
			h := p.GetPrepareResponse().(*prepareResponse).preparationHash
			m.preparationHash = &h
		}
	case dbft.ChangeViewType:
		m.changeViewPayloads = append(m.changeViewPayloads, &changeViewCompact{
			ValidatorIndex:     validator,
			OriginalViewNumber: p.ViewNumber(),
			Timestamp:          p.GetChangeView().(*changeView).timestamp,
			InvocationScript:   p.(*Payload).Witness.InvocationScript,
		})
	case dbft.CommitType:
		m.commitPayloads = append(m.commitPayloads, &commitCompact{
			ValidatorIndex:   validator,
			ViewNumber:       p.ViewNumber(),
			Signature:        p.GetCommit().(*commit).signature,
			InvocationScript: p.(*Payload).Witness.InvocationScript,
		})
	default:
	}
}

// GetPrepareRequest implements the payload.RecoveryMessage interface.
func (m *recoveryMessage) GetPrepareRequest(p dbft.ConsensusPayload[util.Uint256], validators []dbft.PublicKey, primary uint16) dbft.ConsensusPayload[util.Uint256] {
	if m.prepareRequest == nil {
		return nil
	}

	var compact *preparationCompact
	for _, p := range m.preparationPayloads {
		if p != nil && p.ValidatorIndex == uint8(primary) {
			compact = p
			break
		}
	}

	if compact == nil {
		return nil
	}

	req := fromPayload(prepareRequestType, p.(*Payload), m.prepareRequest.payload)
	req.message.ValidatorIndex = byte(primary)
	req.Sender = validators[primary].(*keys.PublicKey).GetScriptHash()
	req.Witness.InvocationScript = compact.InvocationScript
	req.Witness.VerificationScript = getVerificationScript(uint8(primary), validators)

	return req
}

// GetPrepareResponses implements the payload.RecoveryMessage interface.
func (m *recoveryMessage) GetPrepareResponses(p dbft.ConsensusPayload[util.Uint256], validators []dbft.PublicKey) []dbft.ConsensusPayload[util.Uint256] {
	if m.preparationHash == nil {
		return nil
	}

	ps := make([]dbft.ConsensusPayload[util.Uint256], len(m.preparationPayloads))

	for i, resp := range m.preparationPayloads {
		r := fromPayload(prepareResponseType, p.(*Payload), &prepareResponse{
			preparationHash: *m.preparationHash,
		})
		r.message.ValidatorIndex = resp.ValidatorIndex
		r.Sender = validators[resp.ValidatorIndex].(*keys.PublicKey).GetScriptHash()
		r.Witness.InvocationScript = resp.InvocationScript
		r.Witness.VerificationScript = getVerificationScript(resp.ValidatorIndex, validators)

		ps[i] = r
	}

	return ps
}

// GetChangeViews implements the payload.RecoveryMessage interface.
func (m *recoveryMessage) GetChangeViews(p dbft.ConsensusPayload[util.Uint256], validators []dbft.PublicKey) []dbft.ConsensusPayload[util.Uint256] {
	ps := make([]dbft.ConsensusPayload[util.Uint256], len(m.changeViewPayloads))

	for i, cv := range m.changeViewPayloads {
		c := fromPayload(changeViewType, p.(*Payload), &changeView{
			newViewNumber: cv.OriginalViewNumber + 1,
			timestamp:     cv.Timestamp,
		})
		c.message.ViewNumber = cv.OriginalViewNumber
		c.message.ValidatorIndex = cv.ValidatorIndex
		c.Sender = validators[cv.ValidatorIndex].(*keys.PublicKey).GetScriptHash()
		c.Witness.InvocationScript = cv.InvocationScript
		c.Witness.VerificationScript = getVerificationScript(cv.ValidatorIndex, validators)

		ps[i] = c
	}

	return ps
}

// GetPreCommits implements the payload.RecoveryMessage interface. It's a stub
// since N3 doesn't use extension enabling them.
func (m *recoveryMessage) GetPreCommits(p dbft.ConsensusPayload[util.Uint256], validators []dbft.PublicKey) []dbft.ConsensusPayload[util.Uint256] {
	return nil
}

// GetCommits implements the payload.RecoveryMessage interface.
func (m *recoveryMessage) GetCommits(p dbft.ConsensusPayload[util.Uint256], validators []dbft.PublicKey) []dbft.ConsensusPayload[util.Uint256] {
	ps := make([]dbft.ConsensusPayload[util.Uint256], len(m.commitPayloads))

	for i, c := range m.commitPayloads {
		cc := fromPayload(commitType, p.(*Payload), &commit{signature: c.Signature})
		cc.message.ValidatorIndex = c.ValidatorIndex
		cc.Sender = validators[c.ValidatorIndex].(*keys.PublicKey).GetScriptHash()
		cc.Witness.InvocationScript = c.InvocationScript
		cc.Witness.VerificationScript = getVerificationScript(c.ValidatorIndex, validators)

		ps[i] = cc
	}

	return ps
}

// PreparationHash implements the payload.RecoveryMessage interface.
func (m *recoveryMessage) PreparationHash() *util.Uint256 {
	return m.preparationHash
}

func getVerificationScript(i uint8, validators []dbft.PublicKey) []byte {
	if int(i) >= len(validators) {
		return nil
	}

	pub, ok := validators[i].(*keys.PublicKey)
	if !ok {
		return nil
	}

	return pub.GetVerificationScript()
}

func fromPayload(t messageType, recovery *Payload, p io.Serializable) *Payload {
	return &Payload{
		Extensible: npayload.Extensible{
			Category:      npayload.ConsensusCategory,
			ValidBlockEnd: recovery.BlockIndex,
		},
		message: message{
			Type:             t,
			BlockIndex:       recovery.BlockIndex,
			ViewNumber:       recovery.message.ViewNumber,
			payload:          p,
			stateRootEnabled: recovery.stateRootEnabled,
		},
		network: recovery.network,
	}
}
