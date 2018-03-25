package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/crypto"
)

// A Enrollment transaction represents an enrollment form, which indicates
// that the sponsor of the transaction would like to sign up as a validator.
// The way to sign up is: To construct an EnrollmentTransaction type of transaction,
// and send a deposit to the address of the PublicKey.
// The way to cancel the registration is: Spend the deposit on the address of the PublicKey.
type EnrollmentTX struct {
	// PublicKey of the validator
	PublicKey *crypto.PublicKey
}

// DecodeBinary implements the Payload interface.
func (tx *EnrollmentTX) DecodeBinary(r io.Reader) error {
	tx.PublicKey = &crypto.PublicKey{}
	return tx.PublicKey.DecodeBinary(r)
}

// EncodeBinary implements the Payload interface.
func (tx *EnrollmentTX) EncodeBinary(w io.Writer) error {
	return tx.PublicKey.EncodeBinary(w)
}
