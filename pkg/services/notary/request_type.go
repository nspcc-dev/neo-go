package notary

// RequestType represents the type of Notary request.
type RequestType byte

const (
	// Signature represents standard single signature request type.
	Signature RequestType = 0x01
	// MultiSignature represents m out of n multisignature request type.
	MultiSignature RequestType = 0x02
	// Contract represents contract witness type.
	Contract RequestType = 0x03
)
