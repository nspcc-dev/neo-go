package consensus

import (
	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/pkg/errors"
)

type (
	// recoveryMessage represents dBFT Recovery message.
	recoveryMessage struct {
		preparationHash     *util.Uint256
		preparationPayloads []*preparationCompact
		commitPayloads      []*commitCompact
		changeViewPayloads  []*changeViewCompact
		prepareRequest      *message

		stateRootEnabled bool
	}

	changeViewCompact struct {
		ValidatorIndex     uint16
		OriginalViewNumber byte
		Timestamp          uint32
		InvocationScript   []byte
	}

	commitCompact struct {
		ViewNumber       byte
		ValidatorIndex   uint16
		Signature        [signatureSize]byte
		InvocationScript []byte
	}

	preparationCompact struct {
		ValidatorIndex   uint16
		InvocationScript []byte
		StateRootSig     [signatureSize]byte

		stateRootEnabled bool
	}
)

var _ payload.RecoveryMessage = (*recoveryMessage)(nil)

// DecodeBinary implements io.Serializable interface.
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

	lu := r.ReadVarUint()
	if lu > state.MaxValidatorsVoted {
		r.Err = errors.New("too many preparation payloads")
		return
	}
	m.preparationPayloads = make([]*preparationCompact, lu)
	for i := uint64(0); i < lu; i++ {
		m.preparationPayloads[i] = &preparationCompact{stateRootEnabled: m.stateRootEnabled}
		m.preparationPayloads[i].DecodeBinary(r)
	}
	r.ReadArray(&m.commitPayloads)
}

// EncodeBinary implements io.Serializable interface.
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

// DecodeBinary implements io.Serializable interface.
func (p *changeViewCompact) DecodeBinary(r *io.BinReader) {
	p.ValidatorIndex = r.ReadU16LE()
	p.OriginalViewNumber = r.ReadB()
	p.Timestamp = r.ReadU32LE()
	p.InvocationScript = r.ReadVarBytes(1024)
}

// EncodeBinary implements io.Serializable interface.
func (p *changeViewCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteU16LE(p.ValidatorIndex)
	w.WriteB(p.OriginalViewNumber)
	w.WriteU32LE(p.Timestamp)
	w.WriteVarBytes(p.InvocationScript)
}

// DecodeBinary implements io.Serializable interface.
func (p *commitCompact) DecodeBinary(r *io.BinReader) {
	p.ViewNumber = r.ReadB()
	p.ValidatorIndex = r.ReadU16LE()
	r.ReadBytes(p.Signature[:])
	p.InvocationScript = r.ReadVarBytes(1024)
}

// EncodeBinary implements io.Serializable interface.
func (p *commitCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteB(p.ViewNumber)
	w.WriteU16LE(p.ValidatorIndex)
	w.WriteBytes(p.Signature[:])
	w.WriteVarBytes(p.InvocationScript)
}

// DecodeBinary implements io.Serializable interface.
func (p *preparationCompact) DecodeBinary(r *io.BinReader) {
	p.ValidatorIndex = r.ReadU16LE()
	p.InvocationScript = r.ReadVarBytes(1024)
	if p.stateRootEnabled {
		r.ReadBytes(p.StateRootSig[:])
	}
}

// EncodeBinary implements io.Serializable interface.
func (p *preparationCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteU16LE(p.ValidatorIndex)
	w.WriteVarBytes(p.InvocationScript)
	if p.stateRootEnabled {
		w.WriteBytes(p.StateRootSig[:])
	}
}

// AddPayload implements payload.RecoveryMessage interface.
func (m *recoveryMessage) AddPayload(p payload.ConsensusPayload) {
	switch p.Type() {
	case payload.PrepareRequestType:
		m.prepareRequest = &message{
			Type:       prepareRequestType,
			ViewNumber: p.ViewNumber(),
			payload:    p.GetPrepareRequest().(*prepareRequest),

			stateRootEnabled: m.stateRootEnabled,
		}
		h := p.Hash()
		m.preparationHash = &h
		m.preparationPayloads = append(m.preparationPayloads, &preparationCompact{
			stateRootEnabled: m.stateRootEnabled,
			ValidatorIndex:   p.ValidatorIndex(),
			InvocationScript: p.(*Payload).Witness.InvocationScript,
			StateRootSig:     p.GetPrepareRequest().(*prepareRequest).stateRootSig,
		})
	case payload.PrepareResponseType:
		m.preparationPayloads = append(m.preparationPayloads, &preparationCompact{
			stateRootEnabled: m.stateRootEnabled,
			ValidatorIndex:   p.ValidatorIndex(),
			InvocationScript: p.(*Payload).Witness.InvocationScript,
			StateRootSig:     p.GetPrepareResponse().(*prepareResponse).stateRootSig,
		})

		if m.preparationHash == nil {
			h := p.GetPrepareResponse().PreparationHash()
			m.preparationHash = &h
		}
	case payload.ChangeViewType:
		m.changeViewPayloads = append(m.changeViewPayloads, &changeViewCompact{
			ValidatorIndex:     p.ValidatorIndex(),
			OriginalViewNumber: p.ViewNumber(),
			Timestamp:          p.GetChangeView().(*changeView).timestamp,
			InvocationScript:   p.(*Payload).Witness.InvocationScript,
		})
	case payload.CommitType:
		m.commitPayloads = append(m.commitPayloads, &commitCompact{
			ValidatorIndex:   p.ValidatorIndex(),
			ViewNumber:       p.ViewNumber(),
			Signature:        p.GetCommit().(*commit).signature,
			InvocationScript: p.(*Payload).Witness.InvocationScript,
		})
	}
}

// GetPrepareRequest implements payload.RecoveryMessage interface.
func (m *recoveryMessage) GetPrepareRequest(p payload.ConsensusPayload, validators []crypto.PublicKey, primary uint16) payload.ConsensusPayload {
	if m.prepareRequest == nil {
		return nil
	}

	var compact *preparationCompact
	for _, p := range m.preparationPayloads {
		if p != nil && p.ValidatorIndex == primary {
			compact = p
			break
		}
	}

	if compact == nil {
		return nil
	}

	req := fromPayload(prepareRequestType, p.(*Payload), m.prepareRequest.payload)
	req.SetValidatorIndex(primary)
	req.Witness.InvocationScript = compact.InvocationScript
	req.Witness.VerificationScript = getVerificationScript(primary, validators)

	return req
}

// GetPrepareResponses implements payload.RecoveryMessage interface.
func (m *recoveryMessage) GetPrepareResponses(p payload.ConsensusPayload, validators []crypto.PublicKey) []payload.ConsensusPayload {
	if m.preparationHash == nil {
		return nil
	}

	ps := make([]payload.ConsensusPayload, len(m.preparationPayloads))

	for i, resp := range m.preparationPayloads {
		r := fromPayload(prepareResponseType, p.(*Payload), &prepareResponse{
			preparationHash: *m.preparationHash,
			stateRootSig:    resp.StateRootSig,

			stateRootEnabled: m.stateRootEnabled,
		})
		r.SetValidatorIndex(resp.ValidatorIndex)
		r.Witness.InvocationScript = resp.InvocationScript
		r.Witness.VerificationScript = getVerificationScript(resp.ValidatorIndex, validators)

		ps[i] = r
	}

	return ps
}

// GetChangeViews implements payload.RecoveryMessage interface.
func (m *recoveryMessage) GetChangeViews(p payload.ConsensusPayload, validators []crypto.PublicKey) []payload.ConsensusPayload {
	ps := make([]payload.ConsensusPayload, len(m.changeViewPayloads))

	for i, cv := range m.changeViewPayloads {
		c := fromPayload(changeViewType, p.(*Payload), &changeView{
			newViewNumber: cv.OriginalViewNumber + 1,
			timestamp:     cv.Timestamp,
		})
		c.message.ViewNumber = cv.OriginalViewNumber
		c.SetValidatorIndex(cv.ValidatorIndex)
		c.Witness.InvocationScript = cv.InvocationScript
		c.Witness.VerificationScript = getVerificationScript(cv.ValidatorIndex, validators)

		ps[i] = c
	}

	return ps
}

// GetCommits implements payload.RecoveryMessage interface.
func (m *recoveryMessage) GetCommits(p payload.ConsensusPayload, validators []crypto.PublicKey) []payload.ConsensusPayload {
	ps := make([]payload.ConsensusPayload, len(m.commitPayloads))

	for i, c := range m.commitPayloads {
		cc := fromPayload(commitType, p.(*Payload), &commit{
			signature: c.Signature,
		})
		cc.SetValidatorIndex(c.ValidatorIndex)
		cc.Witness.InvocationScript = c.InvocationScript
		cc.Witness.VerificationScript = getVerificationScript(c.ValidatorIndex, validators)

		ps[i] = cc
	}

	return ps
}

// PreparationHash implements payload.RecoveryMessage interface.
func (m *recoveryMessage) PreparationHash() *util.Uint256 {
	return m.preparationHash
}

// SetPreparationHash implements payload.RecoveryMessage interface.
func (m *recoveryMessage) SetPreparationHash(h *util.Uint256) {
	m.preparationHash = h
}

func getVerificationScript(i uint16, validators []crypto.PublicKey) []byte {
	if int(i) >= len(validators) {
		return nil
	}

	pub, ok := validators[i].(*publicKey)
	if !ok {
		return nil
	}

	return pub.GetVerificationScript()
}

func fromPayload(t messageType, recovery *Payload, p io.Serializable) *Payload {
	return &Payload{
		message: &message{
			Type:       t,
			ViewNumber: recovery.message.ViewNumber,
			payload:    p,

			stateRootEnabled: recovery.stateRootEnabled,
		},
		version:  recovery.Version(),
		prevHash: recovery.PrevHash(),
		height:   recovery.Height(),
	}
}
