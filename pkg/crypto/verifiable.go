package crypto

import "github.com/nspcc-dev/neo-go/pkg/util"

// Verifiable represents an object which can be verified.
type Verifiable interface {
	GetSignedPart() []byte
	GetSignedHash() util.Uint256
}

// VerifiableDecodable represents an object which can be both verified and
// decoded from given data.
type VerifiableDecodable interface {
	Verifiable
	DecodeSignedPart([]byte) error
}
