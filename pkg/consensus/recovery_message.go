package consensus

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/pkg/errors"
)

type (
	// recoveryMessage represents dBFT Recovery message.
	recoveryMessage struct {
		PreparationHash     *util.Uint256
		PreparationPayloads []*preparationCompact
		CommitPayloads      []*commitCompact
		ChangeViewPayloads  []*changeViewCompact
		PrepareRequest      *prepareRequest
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
		Signature        []byte
		InvocationScript []byte
	}

	preparationCompact struct {
		ValidatorIndex   uint16
		InvocationScript []byte
	}
)

const uint256size = 32

// DecodeBinary implements io.Serializable interface.
func (m *recoveryMessage) DecodeBinary(r *io.BinReader) {
	m.ChangeViewPayloads = r.ReadArray(changeViewCompact{}).([]*changeViewCompact)

	var hasReq bool
	r.ReadLE(&hasReq)
	if hasReq {
		m.PrepareRequest = new(prepareRequest)
		m.PrepareRequest.DecodeBinary(r)
	} else {
		l := r.ReadVarUint()
		if l != 0 {
			if l == uint256size {
				m.PreparationHash = new(util.Uint256)
				r.ReadBE(m.PreparationHash[:])
			} else {
				r.Err = errors.New("invalid data")
			}
		} else {
			m.PreparationHash = nil
		}
	}

	m.PreparationPayloads = r.ReadArray(preparationCompact{}).([]*preparationCompact)
	m.CommitPayloads = r.ReadArray(commitCompact{}).([]*commitCompact)
}

// EncodeBinary implements io.Serializable interface.
func (m *recoveryMessage) EncodeBinary(w *io.BinWriter) {
	w.WriteArray(m.ChangeViewPayloads)

	hasReq := m.PrepareRequest != nil
	w.WriteLE(hasReq)
	if hasReq {
		m.PrepareRequest.EncodeBinary(w)
	} else {
		if m.PreparationHash == nil {
			w.WriteVarUint(0)
		} else {
			w.WriteVarUint(uint256size)
			w.WriteBE(m.PreparationHash[:])
		}
	}
	w.WriteArray(m.PreparationPayloads)
	w.WriteArray(m.CommitPayloads)
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

	p.Signature = make([]byte, signatureSize)
	r.ReadBE(&p.Signature)
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
