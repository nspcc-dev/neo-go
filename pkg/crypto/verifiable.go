package crypto

// VerifiableDecodable represents an object which can be verified and
// those hashable part can be encoded/decoded.
type VerifiableDecodable interface {
	EncodeHashableFields() ([]byte, error)
	DecodeHashableFields([]byte) error
}
