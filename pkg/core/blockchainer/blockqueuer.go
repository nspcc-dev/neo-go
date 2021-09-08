package blockchainer

import "github.com/nspcc-dev/neo-go/pkg/core/block"

// Blockqueuer is an interface for blockqueue.
type Blockqueuer interface {
	AddBlock(block *block.Block) error
	AddHeaders(...*block.Header) error
	BlockHeight() uint32
}
