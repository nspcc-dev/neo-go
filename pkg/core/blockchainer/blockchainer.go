package blockchainer

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer/services"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Blockchainer is an interface that abstract the implementation
// of the blockchain.
type Blockchainer interface {
	ApplyPolicyToTxSet([]*transaction.Transaction) []*transaction.Transaction
	GetConfig() config.ProtocolConfiguration
	AddHeaders(...*block.Header) error
	AddBlock(*block.Block) error
	CalculateClaimable(h util.Uint160, endHeight uint32) (*big.Int, error)
	Close()
	InitOnRestore(height uint32) error
	InitVerificationVM(v *vm.VM, getContract func(util.Uint160) (*state.Contract, error), hash util.Uint160, witness *transaction.Witness) error
	IsTxStillRelevant(t *transaction.Transaction, txpool *mempool.Pool, isPartialTx bool) bool
	HeaderHeight() uint32
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetCommittee() (keys.PublicKeys, error)
	GetContractState(hash util.Uint160) *state.Contract
	GetContractScriptHash(id int32) (util.Uint160, error)
	GetEnrollments() ([]state.Validator, error)
	GetGoverningTokenBalance(acc util.Uint160) (*big.Int, uint32)
	ForEachNEP17Transfer(util.Uint160, func(*state.NEP17Transfer) (bool, error)) error
	GetHeaderHash(int) util.Uint256
	GetHeader(hash util.Uint256) (*block.Header, error)
	CurrentHeaderHash() util.Uint256
	CurrentBlockHash() util.Uint256
	HasBlock(util.Uint256) bool
	HasTransaction(util.Uint256) bool
	IsExtensibleAllowed(util.Uint160) bool
	GetAppExecResults(util.Uint256, trigger.Type) ([]state.AppExecResult, error)
	GetNotaryDepositExpiration(acc util.Uint160) uint32
	GetNativeContractScriptHash(string) (util.Uint160, error)
	GetNatives() []state.NativeContract
	GetNextBlockValidators() ([]*keys.PublicKey, error)
	GetNEP17Balances(util.Uint160) *state.NEP17Balances
	GetNotaryContractScriptHash() util.Uint160
	GetNotaryBalance(acc util.Uint160) *big.Int
	GetPolicer() Policer
	GetValidators() ([]*keys.PublicKey, error)
	GetStandByCommittee() keys.PublicKeys
	GetStandByValidators() keys.PublicKeys
	GetStateModule() StateRoot
	GetStorageItem(id int32, key []byte) state.StorageItem
	GetStorageItems(id int32) (map[string]state.StorageItem, error)
	GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) *vm.VM
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	SetOracle(service services.Oracle)
	mempool.Feer // fee interface
	ManagementContractHash() util.Uint160
	PoolTx(t *transaction.Transaction, pools ...*mempool.Pool) error
	PoolTxWithData(t *transaction.Transaction, data interface{}, mp *mempool.Pool, feer mempool.Feer, verificationFunction func(bc Blockchainer, t *transaction.Transaction, data interface{}) error) error
	RegisterPostBlock(f func(Blockchainer, *mempool.Pool, *block.Block))
	SetNotary(mod services.Notary)
	SubscribeForBlocks(ch chan<- *block.Block)
	SubscribeForExecutions(ch chan<- *state.AppExecResult)
	SubscribeForNotifications(ch chan<- *state.NotificationEvent)
	SubscribeForTransactions(ch chan<- *transaction.Transaction)
	VerifyTx(*transaction.Transaction) error
	VerifyWitness(util.Uint160, hash.Hashable, *transaction.Witness, int64) error
	GetMemPool() *mempool.Pool
	UnsubscribeFromBlocks(ch chan<- *block.Block)
	UnsubscribeFromExecutions(ch chan<- *state.AppExecResult)
	UnsubscribeFromNotifications(ch chan<- *state.NotificationEvent)
	UnsubscribeFromTransactions(ch chan<- *transaction.Transaction)
}
