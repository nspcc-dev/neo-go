package core

import (
	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/block"
	"github.com/CityOfZion/neo-go/pkg/core/mempool"
	"github.com/CityOfZion/neo-go/pkg/core/state"
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
	AddHeaders(...*block.Header) error
	AddBlock(*block.Block) error
	BlockHeight() uint32
	Close()
	HeaderHeight() uint32
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetContractState(hash util.Uint160) *state.Contract
	GetHeaderHash(int) util.Uint256
	GetHeader(hash util.Uint256) (*block.Header, error)
	CurrentHeaderHash() util.Uint256
	CurrentBlockHash() util.Uint256
	HasBlock(util.Uint256) bool
	HasTransaction(util.Uint256) bool
	GetAssetState(util.Uint256) *state.Asset
	GetAccountState(util.Uint160) *state.Account
	GetValidators(txes ...*transaction.Transaction) ([]*keys.PublicKey, error)
	GetScriptHashesForVerifying(*transaction.Transaction) ([]util.Uint160, error)
	GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem
	GetStorageItems(hash util.Uint160) (map[string]*state.StorageItem, error)
	GetTestVM() (*vm.VM, storage.Store)
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	GetUnspentCoinState(util.Uint256) *UnspentCoinState
	References(t *transaction.Transaction) map[transaction.Input]*transaction.Output
	mempool.Feer // fee interface
	VerifyTx(*transaction.Transaction, *block.Block) error
	GetMemPool() *mempool.Pool
}
