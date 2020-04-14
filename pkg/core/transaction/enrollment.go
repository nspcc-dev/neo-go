package transaction

import (
	"math/rand"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// EnrollmentTX transaction represents an enrollment form, which indicates
// that the sponsor of the transaction would like to sign up as a validator.
// The way to sign up is: To construct an EnrollmentTransaction type of transaction,
// and send a deposit to the address of the PublicKey.
// The way to cancel the registration is: Spend the deposit on the address of the PublicKey.
type EnrollmentTX struct {
	// PublicKey of the validator.
	PublicKey keys.PublicKey
}

// NewEnrollmentTX creates Transaction of EnrollmentType type.
func NewEnrollmentTX(enrollment *EnrollmentTX) *Transaction {
	return &Transaction{
		Type:       EnrollmentType,
		Version:    0,
		Nonce:      rand.Uint32(),
		Data:       enrollment,
		Attributes: []Attribute{},
		Inputs:     []Input{},
		Outputs:    []Output{},
		Scripts:    []Witness{},
		Trimmed:    false,
	}
}

// DecodeBinary implements Serializable interface.
func (tx *EnrollmentTX) DecodeBinary(r *io.BinReader) {
	tx.PublicKey.DecodeBinary(r)
}

// EncodeBinary implements Serializable interface.
func (tx *EnrollmentTX) EncodeBinary(w *io.BinWriter) {
	tx.PublicKey.EncodeBinary(w)
}
