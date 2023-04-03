package interop

import "github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"

const (
	// Hash160Len is the length of proper Hash160 in bytes, use it to
	// sanitize input parameters.
	Hash160Len = 20
	// Hash256Len is the length of proper Hash256 in bytes, use it to
	// sanitize input parameters.
	Hash256Len = 32
	// PublicKeyCompressedLen is the length of compressed public key (which
	// is the most common public key type), use it to sanitize input
	// parameters.
	PublicKeyCompressedLen = 33
	// PublicKeyUncompressedLen is the length of uncompressed public key
	// (but you're not likely to ever encounter that), use it to sanitize
	// input parameters.
	PublicKeyUncompressedLen = 65
	// SignatureLen is the length of standard signature, use it to sanitize
	// input parameters.
	SignatureLen = 64
)

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
type Interface any

// Equals compares Hash160 with the provided stackitem using EQUAL opcode.
// The provided stackitem `b` must be either one of the primitive type (int,
// bool, string, []byte) or derived from the primitive type, otherwise Equals
// will fail on .(string) conversion.
func (a Hash160) Equals(b any) bool {
	ha := any(a)
	return bytesEquals(ha, b)
}

// Equals compares Hash256 with the provided stackitem using EQUAL opcode.
// The provided stackitem `b` must be either one of the primitive type (int,
// bool, string, []byte) or derived from the primitive type, otherwise Equals
// will fail on .(string) conversion.
func (a Hash256) Equals(b any) bool {
	ha := any(a)
	return bytesEquals(ha, b)
}

// Equals compares PublicKey with the provided stackitem using EQUAL opcode.
// The provided stackitem `b` must be either one of the primitive type (int,
// bool, string, []byte) or derived from the primitive type, otherwise Equals
// will fail on .(string) conversion.
func (a PublicKey) Equals(b any) bool {
	ha := any(a)
	return bytesEquals(ha, b)
}

// Equals compares Signature with the provided stackitem using EQUAL opcode.
// The provided stackitem `b` must be either one of the primitive types (int,
// bool, string, []byte) or derived from the primitive type, otherwise Equals
// will fail on .(string) conversion.
func (a Signature) Equals(b any) bool {
	ha := any(a)
	return bytesEquals(ha, b)
}

// bytesEquals is an internal helper function allowed to compare types that can be
// converted to ByteString.
func bytesEquals(a any, b any) bool {
	return (a == nil && b == nil) ||
		(a != nil && b != nil && neogointernal.Opcode2("EQUAL", a.(string), b.(string)).(bool))
}
