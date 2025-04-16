package network

import (
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// StateSync represents state sync module.
type StateSync interface {
	blockHeaderQueuer
	AddMPTNodes([][]byte) error
	AddContractStorageData(key string, value []byte, syncHeight uint32, expectedRoot util.Uint256) error
	Init(currChainHeight uint32) error
	IsActive() bool
	IsInitialized() bool
	GetUnknownMPTNodesBatch(limit int) []util.Uint256
	GetConfig() config.Blockchain
	NeedHeaders() bool
	NeedBlocks() bool
	NeedMPTNodes() bool
	NeedContractStorageData() bool
	SetOnStageChanged(func())
	Traverse(root util.Uint256, process func(node mpt.Node, nodeBytes []byte) bool) error
}
