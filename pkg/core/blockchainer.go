package core

import (
	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// Blockchainer is an interface that abstract the implementation
// of the blockchain.
type Blockchainer interface {
	GetConfig() config.ProtocolConfiguration
	AddHeaders(...*Header) error
	AddBlock(*Block) error
	BlockHeight() uint32
	Close()
	HeaderHeight() uint32
	GetBlock(hash util.Uint256) (*Block, error)
	GetContractState(hash util.Uint160) *ContractState
	GetHeaderHash(int) util.Uint256
	GetHeader(hash util.Uint256) (*Header, error)
	CurrentHeaderHash() util.Uint256
	CurrentBlockHash() util.Uint256
	HasBlock(util.Uint256) bool
	HasTransaction(util.Uint256) bool
	GetAssetState(util.Uint256) *AssetState
	GetAccountState(util.Uint160) *AccountState
	GetValidators(txes... *transaction.Transaction) ([]*keys.PublicKey, error)
	GetScriptHashesForVerifying(*transaction.Transaction) ([]util.Uint160, error)
	GetStorageItem(scripthash util.Uint160, key []byte) *StorageItem
	GetStorageItems(hash util.Uint160) (map[string]*StorageItem, error)
	GetTestVM() (*vm.VM, storage.Store)
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	GetUnspentCoinState(util.Uint256) *UnspentCoinState
	References(t *transaction.Transaction) map[transaction.Input]*transaction.Output
	Feer // fee interface
	VerifyTx(*transaction.Transaction, *Block) error
	GetMemPool() MemPool
}
