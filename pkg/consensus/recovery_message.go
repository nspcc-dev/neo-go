package consensus

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/pkg/errors"
)

type (
	// recoveryMessage represents dBFT Recovery message.
	recoveryMessage struct {
		preparationHash     *util.Uint256
		preparationPayloads []*preparationCompact
		commitPayloads      []*commitCompact
		changeViewPayloads  []*changeViewCompact
		prepareRequest      *prepareRequest
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
	}
)

var _ payload.RecoveryMessage = (*recoveryMessage)(nil)

// DecodeBinary implements io.Serializable interface.
func (m *recoveryMessage) DecodeBinary(r *io.BinReader) {
	r.ReadArray(&m.changeViewPayloads)

	var hasReq bool
	r.ReadLE(&hasReq)
	if hasReq {
		m.prepareRequest = new(prepareRequest)
		m.prepareRequest.DecodeBinary(r)
	} else {
		l := r.ReadVarUint()
		if l != 0 {
			if l == util.Uint256Size {
				m.preparationHash = new(util.Uint256)
				r.ReadBE(m.preparationHash[:])
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

// EncodeBinary implements io.Serializable interface.
func (m *recoveryMessage) EncodeBinary(w *io.BinWriter) {
	w.WriteArray(m.changeViewPayloads)

	hasReq := m.prepareRequest != nil
	w.WriteLE(hasReq)
	if hasReq {
		m.prepareRequest.EncodeBinary(w)
	} else {
		if m.preparationHash == nil {
			w.WriteVarUint(0)
		} else {
			w.WriteVarUint(util.Uint256Size)
			w.WriteBE(m.preparationHash[:])
		}
	}

	w.WriteArray(m.preparationPayloads)
	w.WriteArray(m.commitPayloads)
}

// DecodeBinary implements io.Serializable interface.
func (p *changeViewCompact) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&p.ValidatorIndex)
	r.ReadLE(&p.OriginalViewNumber)
	r.ReadLE(&p.Timestamp)
	p.InvocationScript = r.ReadBytes()
}

// EncodeBinary implements io.Serializable interface.
func (p *changeViewCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(p.ValidatorIndex)
	w.WriteLE(p.OriginalViewNumber)
	w.WriteLE(p.Timestamp)
	w.WriteBytes(p.InvocationScript)
}

// DecodeBinary implements io.Serializable interface.
func (p *commitCompact) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&p.ViewNumber)
	r.ReadLE(&p.ValidatorIndex)
	r.ReadBE(p.Signature[:])
	p.InvocationScript = r.ReadBytes()
}

// EncodeBinary implements io.Serializable interface.
func (p *commitCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(p.ViewNumber)
	w.WriteLE(p.ValidatorIndex)
	w.WriteBE(p.Signature)
	w.WriteBytes(p.InvocationScript)
}

// DecodeBinary implements io.Serializable interface.
func (p *preparationCompact) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&p.ValidatorIndex)
	p.InvocationScript = r.ReadBytes()
}

// EncodeBinary implements io.Serializable interface.
func (p *preparationCompact) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(p.ValidatorIndex)
	w.WriteBytes(p.InvocationScript)
}

// AddPayload implements payload.RecoveryMessage interface.
func (m *recoveryMessage) AddPayload(p payload.ConsensusPayload) {
	switch p.Type() {
	case payload.PrepareRequestType:
		m.prepareRequest = p.GetPrepareRequest().(*prepareRequest)
		h := p.Hash()
		m.preparationHash = &h
	case payload.PrepareResponseType:
		m.preparationPayloads = append(m.preparationPayloads, &preparationCompact{
			ValidatorIndex:   p.ValidatorIndex(),
			InvocationScript: p.(*Payload).Witness.InvocationScript,
		})

		if m.preparationHash == nil {
			h := p.GetPrepareResponse().PreparationHash()
			m.preparationHash = &h
		}
	case payload.ChangeViewType:
		m.changeViewPayloads = append(m.changeViewPayloads, &changeViewCompact{
			ValidatorIndex:     p.ValidatorIndex(),
			OriginalViewNumber: p.ViewNumber(),
			Timestamp:          p.GetChangeView().Timestamp(),
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
func (m *recoveryMessage) GetPrepareRequest(p payload.ConsensusPayload) payload.ConsensusPayload {
	if m.prepareRequest == nil {
		return nil
	}

	return fromPayload(prepareRequestType, p.(*Payload), m.prepareRequest)
}

// GetPrepareResponses implements payload.RecoveryMessage interface.
func (m *recoveryMessage) GetPrepareResponses(p payload.ConsensusPayload) []payload.ConsensusPayload {
	if m.preparationHash == nil {
		return nil
	}

	ps := make([]payload.ConsensusPayload, len(m.preparationPayloads))

	for i, resp := range m.preparationPayloads {
		ps[i] = fromPayload(prepareResponseType, p.(*Payload), &prepareResponse{
			preparationHash: *m.preparationHash,
		})
		ps[i].SetValidatorIndex(resp.ValidatorIndex)
	}

	return ps
}

// GetChangeViews implements payload.RecoveryMessage interface.
func (m *recoveryMessage) GetChangeViews(p payload.ConsensusPayload) []payload.ConsensusPayload {
	ps := make([]payload.ConsensusPayload, len(m.changeViewPayloads))

	for i, cv := range m.changeViewPayloads {
		ps[i] = fromPayload(changeViewType, p.(*Payload), &changeView{
			newViewNumber: cv.OriginalViewNumber + 1,
			timestamp:     cv.Timestamp,
		})
		ps[i].SetValidatorIndex(cv.ValidatorIndex)
	}

	return ps
}

// GetCommits implements payload.RecoveryMessage interface.
func (m *recoveryMessage) GetCommits(p payload.ConsensusPayload) []payload.ConsensusPayload {
	ps := make([]payload.ConsensusPayload, len(m.commitPayloads))

	for i, c := range m.commitPayloads {
		cc := commit{signature: c.Signature}
		ps[i] = fromPayload(commitType, p.(*Payload), &cc)
		ps[i].SetValidatorIndex(c.ValidatorIndex)
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

func fromPayload(t messageType, recovery *Payload, p io.Serializable) *Payload {
	return &Payload{
		message: message{
			Type:       t,
			ViewNumber: recovery.message.ViewNumber,
			payload:    p,
		},
		version:  recovery.Version(),
		prevHash: recovery.PrevHash(),
		height:   recovery.Height(),
	}
}
