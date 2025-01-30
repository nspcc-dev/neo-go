package fakechain

import (
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// FakeChain implements the Blockchainer interface, but does not provide real functionality.
type FakeChain struct {
	config.Blockchain
	*mempool.Pool
	blocksCh                 []chan *block.Block
	Blockheight              atomic.Uint32
	PoolTxF                  func(*transaction.Transaction) error
	poolTxWithData           func(*transaction.Transaction, any, *mempool.Pool) error
	blocks                   map[util.Uint256]*block.Block
	hdrHashes                map[uint32]util.Uint256
	txs                      map[util.Uint256]*transaction.Transaction
	VerifyWitnessF           func() (int64, error)
	MaxVerificationGAS       int64
	NotaryContractScriptHash util.Uint160
	NotaryDepositExpiration  uint32
	PostBlock                []func(func(*transaction.Transaction, *mempool.Pool, bool) bool, *mempool.Pool, *block.Block)
	UtilityTokenBalance      *big.Int
}

// FakeStateSync implements the StateSync interface.
type FakeStateSync struct {
	IsActiveFlag      atomic.Bool
	IsInitializedFlag atomic.Bool
	RequestHeaders    atomic.Bool
	InitFunc          func(h uint32) error
	TraverseFunc      func(root util.Uint256, process func(node mpt.Node, nodeBytes []byte) bool) error
	AddMPTNodesFunc   func(nodes [][]byte) error
}

// HeaderHeight returns the height of the latest stored header.
func (s *FakeStateSync) HeaderHeight() uint32 {
	return 0
}

// NewFakeChain returns a new FakeChain structure.
func NewFakeChain() *FakeChain {
	return NewFakeChainWithCustomCfg(nil)
}

// NewFakeChainWithCustomCfg returns a new FakeChain structure with the specified protocol configuration.
func NewFakeChainWithCustomCfg(protocolCfg func(c *config.Blockchain)) *FakeChain {
	cfg := config.Blockchain{ProtocolConfiguration: config.ProtocolConfiguration{Magic: netmode.UnitTestNet, P2PNotaryRequestPayloadPoolSize: 10}}
	if protocolCfg != nil {
		protocolCfg(&cfg)
	}
	return &FakeChain{
		Pool:           mempool.New(10, 0, false, nil),
		PoolTxF:        func(*transaction.Transaction) error { return nil },
		poolTxWithData: func(*transaction.Transaction, any, *mempool.Pool) error { return nil },
		blocks:         make(map[util.Uint256]*block.Block),
		hdrHashes:      make(map[uint32]util.Uint256),
		txs:            make(map[util.Uint256]*transaction.Transaction),
		Blockchain:     cfg,
	}
}

// PutBlock implements the Blockchainer interface.
func (chain *FakeChain) PutBlock(b *block.Block) {
	chain.blocks[b.Hash()] = b
	chain.hdrHashes[b.Index] = b.Hash()
	chain.Blockheight.Store(b.Index)
}

// PutHeader implements the Blockchainer interface.
func (chain *FakeChain) PutHeader(b *block.Block) {
	chain.hdrHashes[b.Index] = b.Hash()
}

// PutTx implements the Blockchainer interface.
func (chain *FakeChain) PutTx(tx *transaction.Transaction) {
	chain.txs[tx.Hash()] = tx
}

// InitVerificationContext initializes context for witness check.
func (chain *FakeChain) InitVerificationContext(ic *interop.Context, hash util.Uint160, witness *transaction.Witness) error {
	panic("TODO")
}

// IsExtensibleAllowed implements the Blockchainer interface.
func (*FakeChain) IsExtensibleAllowed(uint160 util.Uint160) bool {
	return true
}

// GetNatives implements the blockchainer.Blockchainer interface.
func (*FakeChain) GetNatives() []state.Contract {
	panic("TODO")
}

// GetNotaryDepositExpiration implements the Blockchainer interface.
func (chain *FakeChain) GetNotaryDepositExpiration(acc util.Uint160) uint32 {
	if chain.NotaryDepositExpiration != 0 {
		return chain.NotaryDepositExpiration
	}
	panic("TODO")
}

// GetNotaryContractScriptHash implements the Blockchainer interface.
func (chain *FakeChain) GetNotaryContractScriptHash() util.Uint160 {
	if !chain.NotaryContractScriptHash.Equals(util.Uint160{}) {
		return chain.NotaryContractScriptHash
	}
	panic("TODO")
}

// GetNotaryBalance implements the Blockchainer interface.
func (chain *FakeChain) GetNotaryBalance(acc util.Uint160) *big.Int {
	panic("TODO")
}

// GetNotaryServiceFeePerKey implements the Blockchainer interface.
func (chain *FakeChain) GetNotaryServiceFeePerKey() int64 {
	panic("TODO")
}

// GetBaseExecFee implements the Policer interface.
func (chain *FakeChain) GetBaseExecFee() int64 {
	return interop.DefaultBaseExecFee
}

// GetStoragePrice implements the Policer interface.
func (chain *FakeChain) GetStoragePrice() int64 {
	return native.DefaultStoragePrice
}

// GetMaxVerificationGAS implements the Policer interface.
func (chain *FakeChain) GetMaxVerificationGAS() int64 {
	if chain.MaxVerificationGAS != 0 {
		return chain.MaxVerificationGAS
	}
	panic("TODO")
}

// PoolTxWithData implements the Blockchainer interface.
func (chain *FakeChain) PoolTxWithData(t *transaction.Transaction, data any, mp *mempool.Pool, feer mempool.Feer, verificationFunction func(t *transaction.Transaction, data any) error) error {
	return chain.poolTxWithData(t, data, mp)
}

// RegisterPostBlock implements the Blockchainer interface.
func (chain *FakeChain) RegisterPostBlock(f func(func(*transaction.Transaction, *mempool.Pool, bool) bool, *mempool.Pool, *block.Block)) {
	chain.PostBlock = append(chain.PostBlock, f)
}

// GetConfig implements the Blockchainer interface.
func (chain *FakeChain) GetConfig() config.Blockchain {
	return chain.Blockchain
}

// CalculateClaimable implements the Blockchainer interface.
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

// AddHeaders implements the Blockchainer interface.
func (chain *FakeChain) AddHeaders(...*block.Header) error {
	panic("TODO")
}

// AddBlock implements the Blockchainer interface.
func (chain *FakeChain) AddBlock(block *block.Block) error {
	if block.Index == chain.Blockheight.Load()+1 {
		chain.PutBlock(block)
	}
	return nil
}

// BlockHeight implements the Feer interface.
func (chain *FakeChain) BlockHeight() uint32 {
	return chain.Blockheight.Load()
}

// HeaderHeight implements the Blockchainer interface.
func (chain *FakeChain) HeaderHeight() uint32 {
	return chain.Blockheight.Load()
}

// GetAppExecResults implements the Blockchainer interface.
func (chain *FakeChain) GetAppExecResults(hash util.Uint256, trig trigger.Type) ([]state.AppExecResult, error) {
	panic("TODO")
}

// GetBlock implements the Blockchainer interface.
func (chain *FakeChain) GetBlock(hash util.Uint256) (*block.Block, error) {
	if b, ok := chain.blocks[hash]; ok {
		return b, nil
	}
	return nil, errors.New("not found")
}

// GetCommittee implements the Blockchainer interface.
func (chain *FakeChain) GetCommittee() (keys.PublicKeys, error) {
	panic("TODO")
}

// GetContractState implements the Blockchainer interface.
func (chain *FakeChain) GetContractState(hash util.Uint160) *state.Contract {
	panic("TODO")
}

// GetContractScriptHash implements the Blockchainer interface.
func (chain *FakeChain) GetContractScriptHash(id int32) (util.Uint160, error) {
	panic("TODO")
}

// GetNativeContractScriptHash implements the Blockchainer interface.
func (chain *FakeChain) GetNativeContractScriptHash(name string) (util.Uint160, error) {
	panic("TODO")
}

// GetHeaderHash implements the Blockchainer interface.
func (chain *FakeChain) GetHeaderHash(n uint32) util.Uint256 {
	return chain.hdrHashes[n]
}

// GetHeader implements the Blockchainer interface.
func (chain *FakeChain) GetHeader(hash util.Uint256) (*block.Header, error) {
	b, err := chain.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	return &b.Header, nil
}

// GetNextBlockValidators implements the Blockchainer interface.
func (chain *FakeChain) GetNextBlockValidators() ([]*keys.PublicKey, error) {
	panic("TODO")
}

// GetNEP17Contracts implements the Blockchainer interface.
func (chain *FakeChain) GetNEP11Contracts() []util.Uint160 {
	panic("TODO")
}

// GetNEP17Contracts implements the Blockchainer interface.
func (chain *FakeChain) GetNEP17Contracts() []util.Uint160 {
	panic("TODO")
}

// GetNEP17LastUpdated implements the Blockchainer interface.
func (chain *FakeChain) GetTokenLastUpdated(acc util.Uint160) (map[int32]uint32, error) {
	panic("TODO")
}

// ForEachNEP17Transfer implements the Blockchainer interface.
func (chain *FakeChain) ForEachNEP11Transfer(util.Uint160, uint64, func(*state.NEP11Transfer) (bool, error)) error {
	panic("TODO")
}

// ForEachNEP17Transfer implements the Blockchainer interface.
func (chain *FakeChain) ForEachNEP17Transfer(util.Uint160, uint64, func(*state.NEP17Transfer) (bool, error)) error {
	panic("TODO")
}

// GetValidators implements the Blockchainer interface.
func (chain *FakeChain) GetValidators() ([]*keys.PublicKey, error) {
	panic("TODO")
}

// GetEnrollments implements the Blockchainer interface.
func (chain *FakeChain) GetEnrollments() ([]state.Validator, error) {
	panic("TODO")
}

// GetStorageItem implements the Blockchainer interface.
func (chain *FakeChain) GetStorageItem(id int32, key []byte) state.StorageItem {
	panic("TODO")
}

// GetTestVM implements the Blockchainer interface.
func (chain *FakeChain) GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) (*interop.Context, error) {
	panic("TODO")
}

// CurrentBlockHash implements the Blockchainer interface.
func (chain *FakeChain) CurrentBlockHash() util.Uint256 {
	return util.Uint256{}
}

// HasBlock implements the Blockchainer interface.
func (chain *FakeChain) HasBlock(h util.Uint256) bool {
	_, ok := chain.blocks[h]
	return ok
}

// HasTransaction implements the Blockchainer interface.
func (chain *FakeChain) HasTransaction(h util.Uint256) bool {
	_, ok := chain.txs[h]
	return ok
}

// GetTransaction implements the Blockchainer interface.
func (chain *FakeChain) GetTransaction(h util.Uint256) (*transaction.Transaction, uint32, error) {
	if tx, ok := chain.txs[h]; ok {
		return tx, 1, nil
	}
	return nil, 0, errors.New("not found")
}

// GetMemPool implements the Blockchainer interface.
func (chain *FakeChain) GetMemPool() *mempool.Pool {
	return chain.Pool
}

// GetGoverningTokenBalance implements the Blockchainer interface.
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

// PoolTx implements the Blockchainer interface.
func (chain *FakeChain) PoolTx(tx *transaction.Transaction, _ ...*mempool.Pool) error {
	return chain.PoolTxF(tx)
}

// SubscribeForBlocks implements the Blockchainer interface.
func (chain *FakeChain) SubscribeForBlocks(ch chan *block.Block) {
	chain.blocksCh = append(chain.blocksCh, ch)
}

// SubscribeForExecutions implements the Blockchainer interface.
func (chain *FakeChain) SubscribeForExecutions(ch chan *state.AppExecResult) {
	panic("TODO")
}

// SubscribeForNotifications implements the Blockchainer interface.
func (chain *FakeChain) SubscribeForNotifications(ch chan *state.ContainedNotificationEvent) {
	panic("TODO")
}

// SubscribeForTransactions implements the Blockchainer interface.
func (chain *FakeChain) SubscribeForTransactions(ch chan *transaction.Transaction) {
	panic("TODO")
}

// VerifyTx implements the Blockchainer interface.
func (chain *FakeChain) VerifyTx(*transaction.Transaction) error {
	panic("TODO")
}

// VerifyWitness implements the Blockchainer interface.
func (chain *FakeChain) VerifyWitness(util.Uint160, hash.Hashable, *transaction.Witness, int64) (int64, error) {
	if chain.VerifyWitnessF != nil {
		return chain.VerifyWitnessF()
	}
	panic("TODO")
}

// UnsubscribeFromBlocks implements the Blockchainer interface.
func (chain *FakeChain) UnsubscribeFromBlocks(ch chan *block.Block) {
	for i, c := range chain.blocksCh {
		if c == ch {
			if i < len(chain.blocksCh) {
				copy(chain.blocksCh[i:], chain.blocksCh[i+1:])
			}
			chain.blocksCh = chain.blocksCh[:len(chain.blocksCh)]
		}
	}
}

// UnsubscribeFromExecutions implements the Blockchainer interface.
func (chain *FakeChain) UnsubscribeFromExecutions(ch chan *state.AppExecResult) {
	panic("TODO")
}

// UnsubscribeFromNotifications implements the Blockchainer interface.
func (chain *FakeChain) UnsubscribeFromNotifications(ch chan *state.ContainedNotificationEvent) {
	panic("TODO")
}

// UnsubscribeFromTransactions implements the Blockchainer interface.
func (chain *FakeChain) UnsubscribeFromTransactions(ch chan *transaction.Transaction) {
	panic("TODO")
}

// AddBlock implements the StateSync interface.
func (s *FakeStateSync) AddBlock(block *block.Block) error {
	panic("TODO")
}

// AddHeaders implements the StateSync interface.
func (s *FakeStateSync) AddHeaders(...*block.Header) error {
	panic("TODO")
}

// AddMPTNodes implements the StateSync interface.
func (s *FakeStateSync) AddMPTNodes(nodes [][]byte) error {
	if s.AddMPTNodesFunc != nil {
		return s.AddMPTNodesFunc(nodes)
	}
	panic("TODO")
}

// BlockHeight implements the StateSync interface.
func (s *FakeStateSync) BlockHeight() uint32 {
	return 0
}

// IsActive implements the StateSync interface.
func (s *FakeStateSync) IsActive() bool { return s.IsActiveFlag.Load() }

// IsInitialized implements the StateSync interface.
func (s *FakeStateSync) IsInitialized() bool {
	return s.IsInitializedFlag.Load()
}

// Init implements the StateSync interface.
func (s *FakeStateSync) Init(currChainHeight uint32) error {
	if s.InitFunc != nil {
		return s.InitFunc(currChainHeight)
	}
	panic("TODO")
}

// NeedHeaders implements the StateSync interface.
func (s *FakeStateSync) NeedHeaders() bool { return s.RequestHeaders.Load() }

// NeedBlocks implements the StateSync interface.
func (s *FakeStateSync) NeedBlocks() bool { return false }

// NeedMPTNodes implements the StateSync interface.
func (s *FakeStateSync) NeedMPTNodes() bool {
	panic("TODO")
}

// Traverse implements the StateSync interface.
func (s *FakeStateSync) Traverse(root util.Uint256, process func(node mpt.Node, nodeBytes []byte) bool) error {
	if s.TraverseFunc != nil {
		return s.TraverseFunc(root, process)
	}
	panic("TODO")
}

// GetUnknownMPTNodesBatch implements the StateSync interface.
func (s *FakeStateSync) GetUnknownMPTNodesBatch(limit int) []util.Uint256 {
	panic("TODO")
}

// GetConfig implements the StateSync interface.
func (s *FakeStateSync) GetConfig() config.Blockchain {
	panic("TODO")
}

// SetOnStageChanged implements the StateSync interface.
func (s *FakeStateSync) SetOnStageChanged(func()) {
	panic("TODO")
}
