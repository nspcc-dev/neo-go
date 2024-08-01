package consensus

import (
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
