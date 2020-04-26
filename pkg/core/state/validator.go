package state

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Validator holds the state of a validator.
type Validator struct {
	PublicKey  *keys.PublicKey
	Registered bool
	Votes      util.Fixed8
}

// RegisteredAndHasVotes returns true or false whether Validator is registered and has votes.
func (vs *Validator) RegisteredAndHasVotes() bool {
	return vs.Registered && vs.Votes > util.Fixed8(0)
}

// UnregisteredAndHasNoVotes returns true when Validator is not registered and has no votes.
func (vs *Validator) UnregisteredAndHasNoVotes() bool {
	return !vs.Registered && vs.Votes == 0
}

// EncodeBinary encodes Validator to the given BinWriter.
func (vs *Validator) EncodeBinary(bw *io.BinWriter) {
	vs.PublicKey.EncodeBinary(bw)
	bw.WriteBool(vs.Registered)
	vs.Votes.EncodeBinary(bw)
}

// DecodeBinary decodes Validator from the given BinReader.
func (vs *Validator) DecodeBinary(reader *io.BinReader) {
	vs.PublicKey = &keys.PublicKey{}
	vs.PublicKey.DecodeBinary(reader)
	vs.Registered = reader.ReadBool()
	vs.Votes.DecodeBinary(reader)
}
