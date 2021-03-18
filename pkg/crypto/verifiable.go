package crypto

import "github.com/nspcc-dev/neo-go/pkg/util"

// Verifiable represents an object which can be verified.
type Verifiable interface {
	GetSignedPart() []byte
	GetSignedHash() util.Uint256
}

// VerifiableDecodable represents an object which can be verified and
// those hashable part can be encoded/decoded.
type VerifiableDecodable interface {
	Verifiable
	EncodeHashableFields() ([]byte, error)
	DecodeHashableFields([]byte) error
}
