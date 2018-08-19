package block

import (
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
)

// GetTransactionCount returns the number of transactions that are recorded in
// the given block.
func GetTransactionCount(block *core.Block) int { return 0 }

// GetTransactions returns a list of transactions that are recorded in this block.
func GetTransactions(block *core.Block) []*transaction.Transaction { return nil }

// GetIndex returns the index of the given block.
func GetIndex(block *core.Block) uint32 { return 0 }

// GetHash returns the hash of the given block.
func GetHash(block *core.Block) []byte { return nil }

// GetHash returns the version of the given block.
func GetVersion(block *core.Block) uint32 { return 0 }

// GetHash returns the previous hash of the given block.
func GetPrevHash(block *core.Block) []byte { return nil }

// GetHash returns the merkle root of the given block.
func GetMerkleRoot(block *core.Block) []byte { return nil }

// GetHash returns the timestamp of the given block.
func GetTimestamp(block *core.Block) uint32 { return 0 }

// GetHash returns the next validator address of the given block.
func GetNextConsensus(block *core.Block) []byte { return nil }

// GetConsensusData returns the consensus data of the given block.
func GetConsensusData(block *core.Block) uint64 { return 0 }

// GetTransaction returns a specific transaction that is recorded in the given block
// by the given index.
func GetTransaction(block *core.Block, index int) *transaction.Transaction { return nil }
