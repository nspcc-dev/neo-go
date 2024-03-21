package consensus

import (
	"crypto/sha256"
	"errors"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
)

// privateKey is a wrapper around keys.PrivateKey
// which implements the crypto.PrivateKey interface.
type privateKey struct {
	*keys.PrivateKey
}

var _ dbft.PrivateKey = &privateKey{}

// Sign implements the dbft's crypto.PrivateKey interface.
func (p *privateKey) Sign(data []byte) ([]byte, error) {
	return p.PrivateKey.Sign(data), nil
}

// publicKey is a wrapper around keys.PublicKey
// which implements the crypto.PublicKey interface.
type publicKey struct {
	*keys.PublicKey
}

var _ dbft.PublicKey = &publicKey{}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (p publicKey) MarshalBinary() (data []byte, err error) {
	return p.PublicKey.Bytes(), nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface.
func (p *publicKey) UnmarshalBinary(data []byte) error {
	return p.PublicKey.DecodeBytes(data)
}

// Verify implements the crypto.PublicKey interface.
func (p publicKey) Verify(msg, sig []byte) error {
	hash := sha256.Sum256(msg)
	if p.PublicKey.Verify(sig, hash[:]) {
		return nil
	}

	return errors.New("error")
}
