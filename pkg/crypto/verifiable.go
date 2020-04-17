package crypto

// Verifiable represents an object which can be verified.
type Verifiable interface {
	GetSignedPart() []byte
}
