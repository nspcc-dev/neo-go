package core

import (
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Blockchainer is an interface that abstract the implementation
// of the blockchain.
type Blockchainer interface {
	ApplyPolicyToTxSet([]mempool.TxWithFee) []mempool.TxWithFee
	GetConfig() config.ProtocolConfiguration
	AddHeaders(...*block.Header) error
	AddBlock(*block.Block) error
	AddStateRoot(r *state.MPTRoot) error
	CalculateClaimable(value util.Fixed8, startHeight, endHeight uint32) (util.Fixed8, util.Fixed8, error)
	Close()
	HeaderHeight() uint32
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetContractState(hash util.Uint160) *state.Contract
	GetEnrollments() ([]*state.Validator, error)
	ForEachNEP5Transfer(util.Uint160, *state.NEP5Transfer, func() (bool, error)) error
	ForEachTransfer(util.Uint160, *state.Transfer, func() (bool, error)) error
	GetHeaderHash(int) util.Uint256
	GetHeader(hash util.Uint256) (*block.Header, error)
	CurrentHeaderHash() util.Uint256
	CurrentBlockHash() util.Uint256
	HasBlock(util.Uint256) bool
	HasTransaction(util.Uint256) bool
	GetAssetState(util.Uint256) *state.Asset
	GetAccountState(util.Uint160) *state.Account
	GetAppExecResult(util.Uint256) (*state.AppExecResult, error)
	GetNEP5Metadata(util.Uint160) (*state.NEP5Metadata, error)
	GetNEP5Balances(util.Uint160) *state.NEP5Balances
	GetValidators(txes ...*transaction.Transaction) ([]*keys.PublicKey, error)
	GetScriptHashesForVerifying(*transaction.Transaction) ([]util.Uint160, error)
	GetStateProof(root util.Uint256, key []byte) ([][]byte, error)
	GetStateRoot(height uint32) (*state.MPTRootState, error)
	GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem
	GetStorageItems(hash util.Uint160) (map[string]*state.StorageItem, error)
	GetSystemFeeAmount(h util.Uint256) uint32
	GetTestVM(tx *transaction.Transaction) *vm.VM
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	GetUnspentCoinState(util.Uint256) *state.UnspentCoin
	References(t *transaction.Transaction) ([]transaction.InOut, error)
	mempool.Feer // fee interface
	PoolTx(*transaction.Transaction) error
	StateHeight() uint32
	SubscribeForBlocks(ch chan<- *block.Block)
	SubscribeForExecutions(ch chan<- *state.AppExecResult)
	SubscribeForNotifications(ch chan<- *state.NotificationEvent)
	SubscribeForTransactions(ch chan<- *transaction.Transaction)
	VerifyTx(*transaction.Transaction, *block.Block) error
	GetMemPool() *mempool.Pool
	UnsubscribeFromBlocks(ch chan<- *block.Block)
	UnsubscribeFromExecutions(ch chan<- *state.AppExecResult)
	UnsubscribeFromNotifications(ch chan<- *state.NotificationEvent)
	UnsubscribeFromTransactions(ch chan<- *transaction.Transaction)
}
