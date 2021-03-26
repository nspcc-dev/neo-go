package fakechain

import (
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer/services"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// FakeChain implements Blockchainer interface, but does not provide real functionality.
type FakeChain struct {
	config.ProtocolConfiguration
	*mempool.Pool
	blocksCh                 []chan<- *block.Block
	Blockheight              uint32
	PoolTxF                  func(*transaction.Transaction) error
	poolTxWithData           func(*transaction.Transaction, interface{}, *mempool.Pool) error
	blocks                   map[util.Uint256]*block.Block
	hdrHashes                map[uint32]util.Uint256
	txs                      map[util.Uint256]*transaction.Transaction
	VerifyWitnessF           func() error
	MaxVerificationGAS       int64
	NotaryContractScriptHash util.Uint160
	NotaryDepositExpiration  uint32
	PostBlock                []func(blockchainer.Blockchainer, *mempool.Pool, *block.Block)
	UtilityTokenBalance      *big.Int
}

// NewFakeChain returns new FakeChain structure.
func NewFakeChain() *FakeChain {
	return &FakeChain{
		Pool:                  mempool.New(10, 0, false),
		PoolTxF:               func(*transaction.Transaction) error { return nil },
		poolTxWithData:        func(*transaction.Transaction, interface{}, *mempool.Pool) error { return nil },
		blocks:                make(map[util.Uint256]*block.Block),
		hdrHashes:             make(map[uint32]util.Uint256),
		txs:                   make(map[util.Uint256]*transaction.Transaction),
		ProtocolConfiguration: config.ProtocolConfiguration{Magic: netmode.UnitTestNet, P2PNotaryRequestPayloadPoolSize: 10},
	}
}

// PutBlock implements Blockchainer interface.
func (chain *FakeChain) PutBlock(b *block.Block) {
	chain.blocks[b.Hash()] = b
	chain.hdrHashes[b.Index] = b.Hash()
	atomic.StoreUint32(&chain.Blockheight, b.Index)
}

// PutHeader implements Blockchainer interface.
func (chain *FakeChain) PutHeader(b *block.Block) {
	chain.hdrHashes[b.Index] = b.Hash()
}

// PutTx implements Blockchainer interface.
func (chain *FakeChain) PutTx(tx *transaction.Transaction) {
	chain.txs[tx.Hash()] = tx
}

// ApplyPolicyToTxSet implements Blockchainer interface.
func (chain *FakeChain) ApplyPolicyToTxSet([]*transaction.Transaction) []*transaction.Transaction {
	panic("TODO")
}

// IsTxStillRelevant implements Blockchainer interface.
func (chain *FakeChain) IsTxStillRelevant(t *transaction.Transaction, txpool *mempool.Pool, isPartialTx bool) bool {
	panic("TODO")
}

// InitVerificationVM initializes VM for witness check.
func (chain *FakeChain) InitVerificationVM(v *vm.VM, getContract func(util.Uint160) (*state.Contract, error), hash util.Uint160, witness *transaction.Witness) error {
	panic("TODO")
}

// IsExtensibleAllowed implements Blockchainer interface.
func (*FakeChain) IsExtensibleAllowed(uint160 util.Uint160) bool {
	return true
}

// GetNatives implements blockchainer.Blockchainer interface.
func (*FakeChain) GetNatives() []state.NativeContract {
	panic("TODO")
}

// GetNotaryDepositExpiration implements Blockchainer interface.
func (chain *FakeChain) GetNotaryDepositExpiration(acc util.Uint160) uint32 {
	if chain.NotaryDepositExpiration != 0 {
		return chain.NotaryDepositExpiration
	}
	panic("TODO")
}

// GetNotaryContractScriptHash implements Blockchainer interface.
func (chain *FakeChain) GetNotaryContractScriptHash() util.Uint160 {
	if !chain.NotaryContractScriptHash.Equals(util.Uint160{}) {
		return chain.NotaryContractScriptHash
	}
	panic("TODO")
}

// GetNotaryBalance implements Blockchainer interface.
func (chain *FakeChain) GetNotaryBalance(acc util.Uint160) *big.Int {
	panic("TODO")
}

// GetPolicer implements Blockchainer interface.
func (chain *FakeChain) GetPolicer() blockchainer.Policer {
	return chain
}

// GetBaseExecFee implements Policer interface.
func (chain *FakeChain) GetBaseExecFee() int64 {
	return interop.DefaultBaseExecFee
}

// GetStoragePrice implements Policer interface.
func (chain *FakeChain) GetStoragePrice() int64 {
	return native.DefaultStoragePrice
}

// GetMaxVerificationGAS implements Policer interface.
func (chain *FakeChain) GetMaxVerificationGAS() int64 {
	if chain.MaxVerificationGAS != 0 {
		return chain.MaxVerificationGAS
	}
	panic("TODO")
}

// PoolTxWithData implements Blockchainer interface.
func (chain *FakeChain) PoolTxWithData(t *transaction.Transaction, data interface{}, mp *mempool.Pool, feer mempool.Feer, verificationFunction func(bc blockchainer.Blockchainer, t *transaction.Transaction, data interface{}) error) error {
	return chain.poolTxWithData(t, data, mp)
}

// RegisterPostBlock implements Blockchainer interface.
func (chain *FakeChain) RegisterPostBlock(f func(blockchainer.Blockchainer, *mempool.Pool, *block.Block)) {
	chain.PostBlock = append(chain.PostBlock, f)
}

// GetConfig implements Blockchainer interface.
func (chain *FakeChain) GetConfig() config.ProtocolConfiguration {
	return chain.ProtocolConfiguration
}

// CalculateClaimable implements Blockchainer interface.
func (chain *FakeChain) CalculateClaimable(util.Uint160, uint32) (*big.Int, error) {
	panic("TODO")
}

// FeePerByte implements Feer interface.
func (chain *FakeChain) FeePerByte() int64 {
	panic("TODO")
}

// P2PSigExtensionsEnabled implements Feer interface.
func (chain *FakeChain) P2PSigExtensionsEnabled() bool {
	return true
}

// AddHeaders implements Blockchainer interface.
func (chain *FakeChain) AddHeaders(...*block.Header) error {
	panic("TODO")
}

// AddBlock implements Blockchainer interface.
func (chain *FakeChain) AddBlock(block *block.Block) error {
	if block.Index == atomic.LoadUint32(&chain.Blockheight)+1 {
		chain.PutBlock(block)
	}
	return nil
}

// BlockHeight implements Feer interface.
func (chain *FakeChain) BlockHeight() uint32 {
	return atomic.LoadUint32(&chain.Blockheight)
}

// Close implements Blockchainer interface.
func (chain *FakeChain) Close() {
	panic("TODO")
}

// HeaderHeight implements Blockchainer interface.
func (chain *FakeChain) HeaderHeight() uint32 {
	return atomic.LoadUint32(&chain.Blockheight)
}

// GetAppExecResults implements Blockchainer interface.
func (chain *FakeChain) GetAppExecResults(hash util.Uint256, trig trigger.Type) ([]state.AppExecResult, error) {
	panic("TODO")
}

// GetBlock implements Blockchainer interface.
func (chain *FakeChain) GetBlock(hash util.Uint256) (*block.Block, error) {
	if b, ok := chain.blocks[hash]; ok {
		return b, nil
	}
	return nil, errors.New("not found")
}

// GetCommittee implements Blockchainer interface.
func (chain *FakeChain) GetCommittee() (keys.PublicKeys, error) {
	panic("TODO")
}

// GetContractState implements Blockchainer interface.
func (chain *FakeChain) GetContractState(hash util.Uint160) *state.Contract {
	panic("TODO")
}

// GetContractScriptHash implements Blockchainer interface.
func (chain *FakeChain) GetContractScriptHash(id int32) (util.Uint160, error) {
	panic("TODO")
}

// GetNativeContractScriptHash implements Blockchainer interface.
func (chain *FakeChain) GetNativeContractScriptHash(name string) (util.Uint160, error) {
	panic("TODO")
}

// GetHeaderHash implements Blockchainer interface.
func (chain *FakeChain) GetHeaderHash(n int) util.Uint256 {
	return chain.hdrHashes[uint32(n)]
}

// GetHeader implements Blockchainer interface.
func (chain *FakeChain) GetHeader(hash util.Uint256) (*block.Header, error) {
	b, err := chain.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	return &b.Header, nil
}

// GetNextBlockValidators implements Blockchainer interface.
func (chain *FakeChain) GetNextBlockValidators() ([]*keys.PublicKey, error) {
	panic("TODO")
}

// ForEachNEP17Transfer implements Blockchainer interface.
func (chain *FakeChain) ForEachNEP17Transfer(util.Uint160, func(*state.NEP17Transfer) (bool, error)) error {
	panic("TODO")
}

// GetNEP17Balances implements Blockchainer interface.
func (chain *FakeChain) GetNEP17Balances(util.Uint160) *state.NEP17Balances {
	panic("TODO")
}

// GetValidators implements Blockchainer interface.
func (chain *FakeChain) GetValidators() ([]*keys.PublicKey, error) {
	panic("TODO")
}

// GetStandByCommittee implements Blockchainer interface.
func (chain *FakeChain) GetStandByCommittee() keys.PublicKeys {
	panic("TODO")
}

// GetStandByValidators implements Blockchainer interface.
func (chain *FakeChain) GetStandByValidators() keys.PublicKeys {
	panic("TODO")
}

// GetEnrollments implements Blockchainer interface.
func (chain *FakeChain) GetEnrollments() ([]state.Validator, error) {
	panic("TODO")
}

// GetStateModule implements Blockchainer interface.
func (chain *FakeChain) GetStateModule() blockchainer.StateRoot {
	return nil
}

// GetStorageItem implements Blockchainer interface.
func (chain *FakeChain) GetStorageItem(id int32, key []byte) state.StorageItem {
	panic("TODO")
}

// GetTestVM implements Blockchainer interface.
func (chain *FakeChain) GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) *vm.VM {
	panic("TODO")
}

// GetStorageItems implements Blockchainer interface.
func (chain *FakeChain) GetStorageItems(id int32) (map[string]state.StorageItem, error) {
	panic("TODO")
}

// CurrentHeaderHash implements Blockchainer interface.
func (chain *FakeChain) CurrentHeaderHash() util.Uint256 {
	return util.Uint256{}
}

// CurrentBlockHash implements Blockchainer interface.
func (chain *FakeChain) CurrentBlockHash() util.Uint256 {
	return util.Uint256{}
}

// HasBlock implements Blockchainer interface.
func (chain *FakeChain) HasBlock(h util.Uint256) bool {
	_, ok := chain.blocks[h]
	return ok
}

// HasTransaction implements Blockchainer interface.
func (chain *FakeChain) HasTransaction(h util.Uint256) bool {
	_, ok := chain.txs[h]
	return ok
}

// GetTransaction implements Blockchainer interface.
func (chain *FakeChain) GetTransaction(h util.Uint256) (*transaction.Transaction, uint32, error) {
	if tx, ok := chain.txs[h]; ok {
		return tx, 1, nil
	}
	return nil, 0, errors.New("not found")
}

// GetMemPool implements Blockchainer interface.
func (chain *FakeChain) GetMemPool() *mempool.Pool {
	return chain.Pool
}

// GetGoverningTokenBalance implements Blockchainer interface.
func (chain *FakeChain) GetGoverningTokenBalance(acc util.Uint160) (*big.Int, uint32) {
	panic("TODO")
}

// GetUtilityTokenBalance implements Feer interface.
func (chain *FakeChain) GetUtilityTokenBalance(uint160 util.Uint160) *big.Int {
	if chain.UtilityTokenBalance != nil {
		return chain.UtilityTokenBalance
	}
	panic("TODO")
}

// ManagementContractHash implements Blockchainer interface.
func (chain FakeChain) ManagementContractHash() util.Uint160 {
	panic("TODO")
}

// PoolTx implements Blockchainer interface.
func (chain *FakeChain) PoolTx(tx *transaction.Transaction, _ ...*mempool.Pool) error {
	return chain.PoolTxF(tx)
}

// SetOracle implements Blockchainer interface.
func (chain FakeChain) SetOracle(services.Oracle) {
	panic("TODO")
}

// SetNotary implements Blockchainer interface.
func (chain *FakeChain) SetNotary(notary services.Notary) {
	panic("TODO")
}

// SubscribeForBlocks implements Blockchainer interface.
func (chain *FakeChain) SubscribeForBlocks(ch chan<- *block.Block) {
	chain.blocksCh = append(chain.blocksCh, ch)
}

// SubscribeForExecutions implements Blockchainer interface.
func (chain *FakeChain) SubscribeForExecutions(ch chan<- *state.AppExecResult) {
	panic("TODO")
}

// SubscribeForNotifications implements Blockchainer interface.
func (chain *FakeChain) SubscribeForNotifications(ch chan<- *state.NotificationEvent) {
	panic("TODO")
}

// SubscribeForTransactions implements Blockchainer interface.
func (chain *FakeChain) SubscribeForTransactions(ch chan<- *transaction.Transaction) {
	panic("TODO")
}

// VerifyTx implements Blockchainer interface.
func (chain *FakeChain) VerifyTx(*transaction.Transaction) error {
	panic("TODO")
}

// VerifyWitness implements Blockchainer interface.
func (chain *FakeChain) VerifyWitness(util.Uint160, hash.Hashable, *transaction.Witness, int64) error {
	if chain.VerifyWitnessF != nil {
		return chain.VerifyWitnessF()
	}
	panic("TODO")
}

// UnsubscribeFromBlocks implements Blockchainer interface.
func (chain *FakeChain) UnsubscribeFromBlocks(ch chan<- *block.Block) {
	for i, c := range chain.blocksCh {
		if c == ch {
			if i < len(chain.blocksCh) {
				copy(chain.blocksCh[i:], chain.blocksCh[i+1:])
			}
			chain.blocksCh = chain.blocksCh[:len(chain.blocksCh)]
		}
	}
}

// UnsubscribeFromExecutions implements Blockchainer interface.
func (chain *FakeChain) UnsubscribeFromExecutions(ch chan<- *state.AppExecResult) {
	panic("TODO")
}

// UnsubscribeFromNotifications implements Blockchainer interface.
func (chain *FakeChain) UnsubscribeFromNotifications(ch chan<- *state.NotificationEvent) {
	panic("TODO")
}

// UnsubscribeFromTransactions implements Blockchainer interface.
func (chain *FakeChain) UnsubscribeFromTransactions(ch chan<- *transaction.Transaction) {
	panic("TODO")
}
