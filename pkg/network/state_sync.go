package network

import (
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// StateSync represents state sync module.
type StateSync interface {
	blockHeaderQueuer
	AddMPTNodes([][]byte) error
	AddContractStorageItems(kvs []storage.KeyValue) error
	Init(currChainHeight uint32) error
	InitContractStorageSync(r state.MPTRoot) error
	IsActive() bool
	IsInitialized() bool
	GetUnknownMPTNodesBatch(limit int) []util.Uint256
	GetConfig() config.Blockchain
	GetLastStoredKey() []byte
	NeedHeaders() bool
	NeedBlocks() bool
	NeedStorageData() bool
	GetStateSyncPoint() uint32
	SetOnStageChanged(func())
	Traverse(root util.Uint256, process func(node mpt.Node, nodeBytes []byte) bool) error
	VerifyWitness(h util.Uint160, c hash.Hashable, w *transaction.Witness, gas int64) (int64, error)
}
