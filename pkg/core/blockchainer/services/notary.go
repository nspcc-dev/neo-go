package services

import "github.com/nspcc-dev/neo-go/pkg/crypto/keys"

// Notary is a Notary module interface.
type Notary interface {
	UpdateNotaryNodes(pubs keys.PublicKeys)
}
