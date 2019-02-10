package core

import (
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Blockchainer is an interface that abstract the implementation
// of the blockchain.
type Blockchainer interface {
	AddHeaders(...*Header) error
	AddBlock(*Block) error
	BlockHeight() uint32
	HeaderHeight() uint32
	GetBlock(hash util.Uint256) (*Block, error)
	GetHeaderHash(int) util.Uint256
	GetHeader(hash util.Uint256) (*Header, error)
	CurrentHeaderHash() util.Uint256
	CurrentBlockHash() util.Uint256
	HasBlock(util.Uint256) bool
	HasTransaction(util.Uint256) bool
	GetAssetState(util.Uint256) *AssetState
	GetAccountState(util.Uint160) *AccountState
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
}
