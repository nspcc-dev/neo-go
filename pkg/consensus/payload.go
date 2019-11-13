package consensus

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
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

		Version        uint32
		ValidatorIndex uint16
		PrevHash       util.Uint256
		Height         uint32
		Timestamp      uint32

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

// EncodeBinaryUnsigned writes payload to w excluding signature.
func (p *Payload) EncodeBinaryUnsigned(w *io.BinWriter) {
	w.WriteLE(p.Version)
	w.WriteBE(p.PrevHash[:])
	w.WriteLE(p.Height)
	w.WriteLE(p.ValidatorIndex)
	w.WriteLE(p.Timestamp)

	ww := io.NewBufBinWriter()
	p.message.EncodeBinary(ww.BinWriter)
	w.WriteBytes(ww.Bytes())
}

// EncodeBinary implements io.Serializable interface.
func (p *Payload) EncodeBinary(w *io.BinWriter) {
	p.EncodeBinaryUnsigned(w)

	w.WriteLE(byte(1))
	p.Witness.EncodeBinary(w)
}

// DecodeBinaryUnsigned reads payload from w excluding signature.
func (p *Payload) DecodeBinaryUnsigned(r *io.BinReader) {
	r.ReadLE(&p.Version)
	r.ReadBE(p.PrevHash[:])
	r.ReadLE(&p.Height)
	r.ReadLE(&p.ValidatorIndex)
	r.ReadLE(&p.Timestamp)

	data := r.ReadBytes()
	rr := io.NewBinReaderFromBuf(data)
	p.message.DecodeBinary(rr)
}

// Hash returns 32-byte message hash.
func (p *Payload) Hash() util.Uint256 {
	w := io.NewBufBinWriter()
	p.EncodeBinaryUnsigned(w.BinWriter)

	return hash.DoubleSha256(w.Bytes())
}

// DecodeBinary implements io.Serializable interface.
func (p *Payload) DecodeBinary(r *io.BinReader) {
	p.DecodeBinaryUnsigned(r)

	var b byte
	r.ReadLE(&b)
	if b != 1 {
		r.Err = errors.New("invalid format")
		return
	}

	p.Witness.DecodeBinary(r)
}

// EncodeBinary implements io.Serializable interface.
func (m *message) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(byte(m.Type))
	w.WriteLE(m.ViewNumber)
	m.payload.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable interface.
func (m *message) DecodeBinary(r *io.BinReader) {
	r.ReadLE((*byte)(&m.Type))
	r.ReadLE(&m.ViewNumber)

	switch m.Type {
	case changeViewType:
		cv := new(changeView)
		// NewViewNumber is not marshaled
		cv.NewViewNumber = m.ViewNumber + 1
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
