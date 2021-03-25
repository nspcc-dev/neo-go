package crypto

import "github.com/nspcc-dev/neo-go/pkg/crypto/hash"

// VerifiableDecodable represents an object which can be verified and
// those hashable part can be encoded/decoded.
type VerifiableDecodable interface {
	hash.Hashable
	EncodeHashableFields() ([]byte, error)
	DecodeHashableFields([]byte) error
}
