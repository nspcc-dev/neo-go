package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
)

// EnrollmentTX transaction represents an enrollment form, which indicates
// that the sponsor of the transaction would like to sign up as a validator.
// The way to sign up is: To construct an EnrollmentTransaction type of transaction,
// and send a deposit to the address of the PublicKey.
// The way to cancel the registration is: Spend the deposit on the address of the PublicKey.
type EnrollmentTX struct {
	// PublicKey of the validator
	PublicKey *keys.PublicKey
}

// DecodeBinary implements the Payload interface.
func (tx *EnrollmentTX) DecodeBinary(r io.Reader) error {
	tx.PublicKey = &keys.PublicKey{}
	return tx.PublicKey.DecodeBinary(r)
}

// EncodeBinary implements the Payload interface.
func (tx *EnrollmentTX) EncodeBinary(w io.Writer) error {
	return tx.PublicKey.EncodeBinary(w)
}

// Size returns serialized binary size for this transaction.
func (tx *EnrollmentTX) Size() int {
	return len(tx.PublicKey.Bytes())
}
