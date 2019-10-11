package core

import (
	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Blockchainer is an interface that abstract the implementation
// of the blockchain.
type Blockchainer interface {
	GetConfig() config.ProtocolConfiguration
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
	GetStorageItem(scripthash util.Uint160, key []byte) *StorageItem
	GetStorageItems(hash util.Uint160) (map[string]*StorageItem, error)
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	References(t *transaction.Transaction) map[transaction.Input]*transaction.Output
	Feer // fee interface
	Verify(t *transaction.Transaction) error
	GetMemPool() MemPool
}
