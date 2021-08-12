package blockchainer

import "github.com/nspcc-dev/neo-go/pkg/core/block"

// Blockqueuer is an interface for blockqueue.
type Blockqueuer interface {
	AddBlock(block *block.Block) error
	BlockHeight() uint32
}
