package network

import (
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/network/bqueue"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// StateSync represents state sync module.
type StateSync interface {
	AddMPTNodes([][]byte) error
	bqueue.Blockqueuer
	Init(currChainHeight uint32) error
	IsActive() bool
	IsInitialized() bool
	GetUnknownMPTNodesBatch(limit int) []util.Uint256
	NeedHeaders() bool
	NeedMPTNodes() bool
	Traverse(root util.Uint256, process func(node mpt.Node, nodeBytes []byte) bool) error
}
