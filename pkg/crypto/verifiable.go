package crypto

// Verifiable represents an object which can be verified.
type Verifiable interface {
	GetSignedPart() []byte
}

// VerifiableDecodable represents an object which can be both verified and
// decoded from given data.
type VerifiableDecodable interface {
	Verifiable
	DecodeSignedPart([]byte) error
}
