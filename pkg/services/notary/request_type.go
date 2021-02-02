package notary

// RequestType represents the type of Notary request.
type RequestType byte

const (
	// Unknown represents unknown request type which means that main tx witnesses are invalid.
	Unknown RequestType = 0x00
	// Signature represents standard single signature request type.
	Signature RequestType = 0x01
	// MultiSignature represents m out of n multisignature request type.
	MultiSignature RequestType = 0x02
)
