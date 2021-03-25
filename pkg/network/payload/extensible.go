package payload

import (
	"errors"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	maxExtensibleCategorySize = 32
	maxExtensibleDataSize     = math.MaxUint16
)

// Extensible represents payload containing arbitrary data.
type Extensible struct {
	// Category is payload type.
	Category string
	// ValidBlockStart is starting height for payload to be valid.
	ValidBlockStart uint32
	// ValidBlockEnd is height after which payload becomes invalid.
	ValidBlockEnd uint32
	// Sender is payload sender or signer.
	Sender util.Uint160
	// Data is custom payload data.
	Data []byte
	// Witness is payload witness.
	Witness transaction.Witness

	hash util.Uint256
}

var errInvalidPadding = errors.New("invalid padding")

// NewExtensible creates new extensible payload.
func NewExtensible() *Extensible {
	return &Extensible{}
}

func (e *Extensible) encodeBinaryUnsigned(w *io.BinWriter) {
	w.WriteString(e.Category)
	w.WriteU32LE(e.ValidBlockStart)
	w.WriteU32LE(e.ValidBlockEnd)
	w.WriteBytes(e.Sender[:])
	w.WriteVarBytes(e.Data)
}

// EncodeBinary implements io.Serializable.
func (e *Extensible) EncodeBinary(w *io.BinWriter) {
	e.encodeBinaryUnsigned(w)
	w.WriteB(1)
	e.Witness.EncodeBinary(w)
}

func (e *Extensible) decodeBinaryUnsigned(r *io.BinReader) {
	e.Category = r.ReadString(maxExtensibleCategorySize)
	e.ValidBlockStart = r.ReadU32LE()
	e.ValidBlockEnd = r.ReadU32LE()
	r.ReadBytes(e.Sender[:])
	e.Data = r.ReadVarBytes(MaxSize)
}

// DecodeBinary implements io.Serializable.
func (e *Extensible) DecodeBinary(r *io.BinReader) {
	e.decodeBinaryUnsigned(r)
	if r.ReadB() != 1 {
		if r.Err != nil {
			return
		}
		r.Err = errInvalidPadding
		return
	}
	e.Witness.DecodeBinary(r)
}

// Hash returns payload hash.
func (e *Extensible) Hash() util.Uint256 {
	if e.hash.Equals(util.Uint256{}) {
		e.createHash()
	}
	return e.hash
}

// createHash creates hashes of the payload.
func (e *Extensible) createHash() {
	buf := io.NewBufBinWriter()
	e.encodeBinaryUnsigned(buf.BinWriter)
	e.hash = hash.Sha256(buf.Bytes())
}
