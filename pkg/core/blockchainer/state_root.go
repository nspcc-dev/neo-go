package blockchainer

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// StateRoot represents local state root module.
type StateRoot interface {
	AddStateRoot(root *state.MPTRoot) error
	CleanStorage() error
	CurrentLocalHeight() uint32
	CurrentLocalStateRoot() util.Uint256
	CurrentValidatedHeight() uint32
	FindStates(root util.Uint256, prefix, start []byte, max int) ([]storage.KeyValue, error)
	GetState(root util.Uint256, key []byte) ([]byte, error)
	GetStateProof(root util.Uint256, key []byte) ([][]byte, error)
	GetStateRoot(height uint32) (*state.MPTRoot, error)
	GetStateValidators(height uint32) keys.PublicKeys
	SetUpdateValidatorsCallback(func(uint32, keys.PublicKeys))
	UpdateStateValidators(height uint32, pubs keys.PublicKeys)
}
