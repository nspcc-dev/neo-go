package management

import "github.com/nspcc-dev/neo-go/pkg/interop"

// Contract represents deployed contract.
type Contract struct {
	ID            int
	UpdateCounter int
	Hash          interop.Hash160
	NEF           []byte
}
