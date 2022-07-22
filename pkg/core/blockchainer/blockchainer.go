package blockchainer

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Blockchainer is an interface that abstracts the implementation
// of the blockchain.
type Blockchainer interface {
	AddBlock(block *block.Block) error
	BlockHeight() uint32
	GetConfig() config.ProtocolConfiguration
	CalculateClaimable(h util.Uint160, endHeight uint32) (*big.Int, error)
	InitVerificationContext(ic *interop.Context, hash util.Uint160, witness *transaction.Witness) error
	HeaderHeight() uint32
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetCommittee() (keys.PublicKeys, error)
	GetContractState(hash util.Uint160) *state.Contract
	GetContractScriptHash(id int32) (util.Uint160, error)
	GetEnrollments() ([]state.Validator, error)
	GetGoverningTokenBalance(acc util.Uint160) (*big.Int, uint32)
	ForEachNEP11Transfer(acc util.Uint160, newestTimestamp uint64, f func(*state.NEP11Transfer) (bool, error)) error
	ForEachNEP17Transfer(acc util.Uint160, newestTimestamp uint64, f func(*state.NEP17Transfer) (bool, error)) error
	GetHeaderHash(int) util.Uint256
	GetHeader(hash util.Uint256) (*block.Header, error)
	CurrentBlockHash() util.Uint256
	GetAppExecResults(util.Uint256, trigger.Type) ([]state.AppExecResult, error)
	GetNativeContractScriptHash(string) (util.Uint160, error)
	GetNatives() []state.NativeContract
	GetNextBlockValidators() ([]*keys.PublicKey, error)
	GetNEP11Contracts() []util.Uint160
	GetNEP17Contracts() []util.Uint160
	GetTokenLastUpdated(acc util.Uint160) (map[int32]uint32, error)
	GetNotaryContractScriptHash() util.Uint160
	GetNotaryServiceFeePerKey() int64
	GetValidators() ([]*keys.PublicKey, error)
	GetStateModule() StateRoot
	GetStorageItem(id int32, key []byte) state.StorageItem
	GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) *interop.Context
	GetTestHistoricVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) (*interop.Context, error)
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	mempool.Feer // fee interface
	SubscribeForBlocks(ch chan<- *block.Block)
	SubscribeForExecutions(ch chan<- *state.AppExecResult)
	SubscribeForNotifications(ch chan<- *state.ContainedNotificationEvent)
	SubscribeForTransactions(ch chan<- *transaction.Transaction)
	VerifyTx(*transaction.Transaction) error
	VerifyWitness(util.Uint160, hash.Hashable, *transaction.Witness, int64) (int64, error)
	GetMemPool() *mempool.Pool
	UnsubscribeFromBlocks(ch chan<- *block.Block)
	UnsubscribeFromExecutions(ch chan<- *state.AppExecResult)
	UnsubscribeFromNotifications(ch chan<- *state.ContainedNotificationEvent)
	UnsubscribeFromTransactions(ch chan<- *transaction.Transaction)
	// Policer.
	GetBaseExecFee() int64
	GetMaxVerificationGAS() int64
	FeePerByte() int64
}
