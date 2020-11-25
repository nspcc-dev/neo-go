package native

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	policyName       = "Policy"
	policyContractID = -3

	defaultMaxBlockSize            = 1024 * 256
	defaultMaxTransactionsPerBlock = 512
	defaultFeePerByte              = 1000
	defaultMaxVerificationGas      = 50000000
	defaultMaxBlockSystemFee       = 9000 * GASFactor
	// minBlockSystemFee is the minimum allowed system fee per block.
	minBlockSystemFee = 4007600
	// maxFeePerByte is the maximum allowed fee per byte value.
	maxFeePerByte = 100_000_000

	// blockedAccountPrefix is a prefix used to store blocked account.
	blockedAccountPrefix = 15
)

var (
	// maxTransactionsPerBlockKey is a key used to store the maximum number of
	// transactions allowed in block.
	maxTransactionsPerBlockKey = []byte{23}
	// feePerByteKey is a key used to store the minimum fee per byte for
	// transaction.
	feePerByteKey = []byte{10}
	// maxBlockSizeKey is a key used to store the maximum block size value.
	maxBlockSizeKey = []byte{12}
	// maxBlockSystemFeeKey is a key used to store the maximum block system fee value.
	maxBlockSystemFeeKey = []byte{17}
)

// Policy represents Policy native contract.
type Policy struct {
	interop.ContractMD
	lock sync.RWMutex
	// isValid defies whether cached values were changed during the current
	// consensus iteration. If false, these values will be updated after
	// blockchain DAO persisting. If true, we can safely use cached values.
	isValid                 bool
	maxTransactionsPerBlock uint32
	maxBlockSize            uint32
	feePerByte              int64
	maxBlockSystemFee       int64
	maxVerificationGas      int64
	blockedAccounts         []util.Uint160
}

var _ interop.Contract = (*Policy)(nil)

// newPolicy returns Policy native contract.
func newPolicy() *Policy {
	p := &Policy{ContractMD: *interop.NewContractMD(policyName)}

	p.ContractID = policyContractID

	desc := newDescriptor("getMaxTransactionsPerBlock", smartcontract.IntegerType)
	md := newMethodAndPrice(p.getMaxTransactionsPerBlock, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("getMaxBlockSize", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getMaxBlockSize, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("getFeePerByte", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getFeePerByte, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("isBlocked", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.isBlocked, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("getMaxBlockSystemFee", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getMaxBlockSystemFee, 1000000, smartcontract.AllowStates)
	p.AddMethod(md, desc, true)

	desc = newDescriptor("setMaxBlockSize", smartcontract.BoolType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxBlockSize, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("setMaxTransactionsPerBlock", smartcontract.BoolType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxTransactionsPerBlock, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("setFeePerByte", smartcontract.BoolType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setFeePerByte, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("setMaxBlockSystemFee", smartcontract.BoolType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxBlockSystemFee, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("blockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.blockAccount, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("unblockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.unblockAccount, 3000000, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("onPersist", smartcontract.VoidType)
	md = newMethodAndPrice(getOnPersistWrapper(p.OnPersist), 0, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)

	desc = newDescriptor("postPersist", smartcontract.VoidType)
	md = newMethodAndPrice(getOnPersistWrapper(postPersistBase), 0, smartcontract.AllowModifyStates)
	p.AddMethod(md, desc, false)
	return p
}

// Metadata implements Contract interface.
func (p *Policy) Metadata() *interop.ContractMD {
	return &p.ContractMD
}

// Initialize initializes Policy native contract and implements Contract interface.
func (p *Policy) Initialize(ic *interop.Context) error {
	p.isValid = true
	p.maxTransactionsPerBlock = defaultMaxTransactionsPerBlock
	p.maxBlockSize = defaultMaxBlockSize
	p.feePerByte = defaultFeePerByte
	p.maxBlockSystemFee = defaultMaxBlockSystemFee
	p.maxVerificationGas = defaultMaxVerificationGas
	p.blockedAccounts = make([]util.Uint160, 0)

	return nil
}

// OnPersist implements Contract interface.
func (p *Policy) OnPersist(ic *interop.Context) error {
	return nil
}

// OnPersistEnd updates cached Policy values if they've been changed
func (p *Policy) OnPersistEnd(dao dao.DAO) error {
	if p.isValid {
		return nil
	}
	p.lock.Lock()
	defer p.lock.Unlock()

	p.maxTransactionsPerBlock = p.getUint32WithKey(dao, maxTransactionsPerBlockKey, defaultMaxTransactionsPerBlock)
	p.maxBlockSize = p.getUint32WithKey(dao, maxBlockSizeKey, defaultMaxBlockSize)
	p.feePerByte = p.getInt64WithKey(dao, feePerByteKey, defaultFeePerByte)
	p.maxBlockSystemFee = p.getInt64WithKey(dao, maxBlockSystemFeeKey, defaultMaxBlockSystemFee)
	p.maxVerificationGas = defaultMaxVerificationGas

	p.blockedAccounts = make([]util.Uint160, 0)
	siMap, err := dao.GetStorageItemsWithPrefix(p.ContractID, []byte{blockedAccountPrefix})
	if err != nil {
		return fmt.Errorf("failed to get blocked accounts from storage: %w", err)
	}
	for key := range siMap {
		hash, err := util.Uint160DecodeBytesBE([]byte(key))
		if err != nil {
			return fmt.Errorf("failed to decode blocked account hash: %w", err)
		}
		p.blockedAccounts = append(p.blockedAccounts, hash)
	}
	sort.Slice(p.blockedAccounts, func(i, j int) bool {
		return p.blockedAccounts[i].Less(p.blockedAccounts[j])
	})

	p.isValid = true
	return nil
}

// getMaxTransactionsPerBlock is Policy contract method and returns the upper
// limit of transactions per block.
func (p *Policy) getMaxTransactionsPerBlock(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetMaxTransactionsPerBlockInternal(ic.DAO))))
}

// GetMaxTransactionsPerBlockInternal returns the upper limit of transactions per
// block.
func (p *Policy) GetMaxTransactionsPerBlockInternal(dao dao.DAO) uint32 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return p.maxTransactionsPerBlock
	}
	return p.getUint32WithKey(dao, maxTransactionsPerBlockKey, defaultMaxTransactionsPerBlock)
}

// getMaxBlockSize is Policy contract method and returns maximum block size.
func (p *Policy) getMaxBlockSize(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetMaxBlockSizeInternal(ic.DAO))))
}

// GetMaxBlockSizeInternal returns maximum block size.
func (p *Policy) GetMaxBlockSizeInternal(dao dao.DAO) uint32 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return p.maxBlockSize
	}
	return p.getUint32WithKey(dao, maxBlockSizeKey, defaultMaxBlockSize)
}

// getFeePerByte is Policy contract method and returns required transaction's fee
// per byte.
func (p *Policy) getFeePerByte(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetFeePerByteInternal(ic.DAO)))
}

// GetFeePerByteInternal returns required transaction's fee per byte.
func (p *Policy) GetFeePerByteInternal(dao dao.DAO) int64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return p.feePerByte
	}
	return p.getInt64WithKey(dao, feePerByteKey, defaultFeePerByte)
}

// GetMaxVerificationGas returns maximum gas allowed to be burned during verificaion.
func (p *Policy) GetMaxVerificationGas(_ dao.DAO) int64 {
	if p.isValid {
		return p.maxVerificationGas
	}
	return defaultMaxVerificationGas
}

// getMaxBlockSystemFee is Policy contract method and returns the maximum overall
// system fee per block.
func (p *Policy) getMaxBlockSystemFee(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetMaxBlockSystemFeeInternal(ic.DAO)))
}

// GetMaxBlockSystemFeeInternal the maximum overall system fee per block.
func (p *Policy) GetMaxBlockSystemFeeInternal(dao dao.DAO) int64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return p.maxBlockSystemFee
	}
	return p.getInt64WithKey(dao, maxBlockSystemFeeKey, defaultMaxBlockSystemFee)
}

// isBlocked is Policy contract method and checks whether provided account is blocked.
func (p *Policy) isBlocked(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	hash := toUint160(args[0])
	return stackitem.NewBool(p.IsBlockedInternal(ic.DAO, hash))
}

// IsBlockedInternal checks whether provided account is blocked
func (p *Policy) IsBlockedInternal(dao dao.DAO, hash util.Uint160) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		length := len(p.blockedAccounts)
		i := sort.Search(length, func(i int) bool {
			return !p.blockedAccounts[i].Less(hash)
		})
		if length != 0 && i != length && p.blockedAccounts[i].Equals(hash) {
			return true
		}
		return false
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	return dao.GetStorageItem(p.ContractID, key) != nil
}

// setMaxTransactionsPerBlock is Policy contract method and  sets the upper limit
// of transactions per block.
func (p *Policy) setMaxTransactionsPerBlock(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := uint32(toBigInt(args[0]).Int64())
	if value > block.MaxTransactionsPerBlock {
		panic(fmt.Errorf("MaxTransactionsPerBlock cannot exceed the maximum allowed transactions per block = %d", block.MaxTransactionsPerBlock))
	}
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err = p.setUint32WithKey(ic.DAO, maxTransactionsPerBlockKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// setMaxBlockSize is Policy contract method and sets maximum block size.
func (p *Policy) setMaxBlockSize(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := uint32(toBigInt(args[0]).Int64())
	if value > payload.MaxSize {
		panic(fmt.Errorf("MaxBlockSize cannot be more than the maximum payload size = %d", payload.MaxSize))
	}
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err = p.setUint32WithKey(ic.DAO, maxBlockSizeKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// setFeePerByte is Policy contract method and sets transaction's fee per byte.
func (p *Policy) setFeePerByte(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toBigInt(args[0]).Int64()
	if value < 0 || value > maxFeePerByte {
		panic(fmt.Errorf("FeePerByte shouldn't be negative or greater than %d", maxFeePerByte))
	}
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err = p.setInt64WithKey(ic.DAO, feePerByteKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// setMaxBlockSystemFee is Policy contract method and sets the maximum system fee per block.
func (p *Policy) setMaxBlockSystemFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toBigInt(args[0]).Int64()
	if value <= minBlockSystemFee {
		panic(fmt.Errorf("MaxBlockSystemFee cannot be less then %d", minBlockSystemFee))
	}
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err = p.setInt64WithKey(ic.DAO, maxBlockSystemFeeKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// blockAccount is Policy contract method and adds given account hash to the list
// of blocked accounts.
func (p *Policy) blockAccount(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	hash := toUint160(args[0])
	if p.IsBlockedInternal(ic.DAO, hash) {
		return stackitem.NewBool(false)
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	p.lock.Lock()
	defer p.lock.Unlock()
	err = ic.DAO.PutStorageItem(p.ContractID, key, &state.StorageItem{
		Value: []byte{0x01},
	})
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

// unblockAccount is Policy contract method and removes given account hash from
// the list of blocked accounts.
func (p *Policy) unblockAccount(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	ok, err := p.checkValidators(ic)
	if err != nil {
		panic(err)
	}
	if !ok {
		return stackitem.NewBool(false)
	}
	hash := toUint160(args[0])
	if !p.IsBlockedInternal(ic.DAO, hash) {
		return stackitem.NewBool(false)
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	p.lock.Lock()
	defer p.lock.Unlock()
	err = ic.DAO.DeleteStorageItem(p.ContractID, key)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
}

func (p *Policy) getUint32WithKey(dao dao.DAO, key []byte, defaultValue uint32) uint32 {
	si := dao.GetStorageItem(p.ContractID, key)
	if si == nil {
		return defaultValue
	}
	return binary.LittleEndian.Uint32(si.Value)
}

func (p *Policy) setUint32WithKey(dao dao.DAO, key []byte, value uint32) error {
	si := &state.StorageItem{
		Value: make([]byte, 4),
	}
	binary.LittleEndian.PutUint32(si.Value, value)
	return dao.PutStorageItem(p.ContractID, key, si)
}

func (p *Policy) getInt64WithKey(dao dao.DAO, key []byte, defaultValue int64) int64 {
	si := dao.GetStorageItem(p.ContractID, key)
	if si == nil {
		return defaultValue
	}
	return int64(binary.LittleEndian.Uint64(si.Value))
}

func (p *Policy) setInt64WithKey(dao dao.DAO, key []byte, value int64) error {
	si := &state.StorageItem{
		Value: make([]byte, 8),
	}
	binary.LittleEndian.PutUint64(si.Value, uint64(value))
	return dao.PutStorageItem(p.ContractID, key, si)
}

func (p *Policy) checkValidators(ic *interop.Context) (bool, error) {
	prevBlock, err := ic.Chain.GetBlock(ic.Block.PrevHash)
	if err != nil {
		return false, err
	}
	return runtime.CheckHashedWitness(ic, prevBlock.NextConsensus)
}

// CheckPolicy checks whether transaction conforms to current policy restrictions
// like not being signed by blocked account or not exceeding block-level system
// fee limit.
func (p *Policy) CheckPolicy(d dao.DAO, tx *transaction.Transaction) error {
	for _, signer := range tx.Signers {
		if p.IsBlockedInternal(d, signer.Account) {
			return fmt.Errorf("account %s is blocked", signer.Account.StringLE())
		}
	}
	maxBlockSystemFee := p.GetMaxBlockSystemFeeInternal(d)
	if maxBlockSystemFee < tx.SystemFee {
		return fmt.Errorf("transaction's fee can't exceed maximum block system fee %d", maxBlockSystemFee)
	}
	return nil
}
