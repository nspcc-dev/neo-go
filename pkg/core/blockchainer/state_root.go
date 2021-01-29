package blockchainer

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// StateRoot represents local state root module.
type StateRoot interface {
	CurrentLocalStateRoot() util.Uint256
	GetStateProof(root util.Uint256, key []byte) ([][]byte, error)
	GetStateRoot(height uint32) (*state.MPTRoot, error)
}
