package interop

// Signature represents 64-byte signature.
type Signature []byte

// Hash160 represents 20-byte hash.
type Hash160 []byte

// Hash256 represents 32-byte hash.
type Hash256 []byte

// PublicKey represents marshalled ecdsa public key.
type PublicKey []byte

// Interface represents interop interface type which is needed for
// transparent handling of VM-internal types (e.g. storage.Context).
type Interface interface{}
