package blockchainer

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
	BlockHeight() uint32
	CalculateClaimable(value int64, startHeight, endHeight uint32) util.Fixed8
	Close()
	HeaderHeight() uint32
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetContractState(hash util.Uint160) *state.Contract
	GetEnrollments() ([]state.Validator, error)
	GetGoverningTokenBalance(acc util.Uint160) (util.Fixed8, uint32)
	GetHeaderHash(int) util.Uint256
	GetHeader(hash util.Uint256) (*block.Header, error)
	CurrentHeaderHash() util.Uint256
	CurrentBlockHash() util.Uint256
	HasBlock(util.Uint256) bool
	HasTransaction(util.Uint256) bool
	GetAssetState(util.Uint256) *state.Asset
	GetAccountState(util.Uint160) *state.Account
	GetAppExecResult(util.Uint256) (*state.AppExecResult, error)
	GetNEP5TransferLog(util.Uint160) *state.NEP5TransferLog
	GetNEP5Balances(util.Uint160) *state.NEP5Balances
	GetValidators() ([]*keys.PublicKey, error)
	GetStandByValidators() (keys.PublicKeys, error)
	GetScriptHashesForVerifying(*transaction.Transaction) ([]util.Uint160, error)
	GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem
	GetStorageItems(hash util.Uint160) (map[string]*state.StorageItem, error)
	GetTestVM() *vm.VM
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	mempool.Feer // fee interface
	PoolTx(*transaction.Transaction) error
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
