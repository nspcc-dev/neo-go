package blockchainer

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// StateRoot represents local state root module.
type StateRoot interface {
	AddStateRoot(root *state.MPTRoot) error
	CurrentLocalStateRoot() util.Uint256
	CurrentValidatedHeight() uint32
	GetStateProof(root util.Uint256, key []byte) ([][]byte, error)
	GetStateRoot(height uint32) (*state.MPTRoot, error)
	UpdateStateValidators(height uint32, pubs keys.PublicKeys)
}
