package native

import "math/big"

// gasIndexPair contains block index together with generated gas per block.
// It is used to cache NEO GASRecords.
type gasIndexPair struct {
	Index       uint32
	GASPerBlock big.Int
}

// gasRecord contains history of gas per block changes. It is used only by NEO cache.
type gasRecord []gasIndexPair
