package native

import (
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	policyContractID = -4

	defaultMaxBlockSize            = 1024 * 256
	defaultMaxTransactionsPerBlock = 512
	defaultExecFeeFactor           = interop.DefaultBaseExecFee
	defaultFeePerByte              = 1000
	defaultMaxVerificationGas      = 50000000
	defaultMaxBlockSystemFee       = 9000 * GASFactor
	// minBlockSystemFee is the minimum allowed system fee per block.
	minBlockSystemFee = 4007600
	// maxExecFeeFactor is the maximum allowed execution fee factor.
	maxExecFeeFactor = 1000
	// maxFeePerByte is the maximum allowed fee per byte value.
	maxFeePerByte = 100_000_000
	// maxStoragePrice is the maximum allowed price for a byte of storage.
	maxStoragePrice = 10000000

	// blockedAccountPrefix is a prefix used to store blocked account.
	blockedAccountPrefix = 15
)

var (
	// maxTransactionsPerBlockKey is a key used to store the maximum number of
	// transactions allowed in block.
	maxTransactionsPerBlockKey = []byte{23}
	// execFeeFactorKey is a key used to store execution fee factor.
	execFeeFactorKey = []byte{18}
	// feePerByteKey is a key used to store the minimum fee per byte for
	// transaction.
	feePerByteKey = []byte{10}
	// maxBlockSizeKey is a key used to store the maximum block size value.
	maxBlockSizeKey = []byte{12}
	// maxBlockSystemFeeKey is a key used to store the maximum block system fee value.
	maxBlockSystemFeeKey = []byte{17}
	// storagePriceKey is a key used to store storage price.
	storagePriceKey = []byte{19}
)

// Policy represents Policy native contract.
type Policy struct {
	interop.ContractMD
	NEO  *NEO
	lock sync.RWMutex
	// isValid defies whether cached values were changed during the current
	// consensus iteration. If false, these values will be updated after
	// blockchain DAO persisting. If true, we can safely use cached values.
	isValid                 bool
	maxTransactionsPerBlock uint32
	maxBlockSize            uint32
	execFeeFactor           uint32
	feePerByte              int64
	maxBlockSystemFee       int64
	maxVerificationGas      int64
	storagePrice            uint32
	blockedAccounts         []util.Uint160
}

var _ interop.Contract = (*Policy)(nil)

// newPolicy returns Policy native contract.
func newPolicy() *Policy {
	p := &Policy{ContractMD: *interop.NewContractMD(nativenames.Policy, policyContractID)}

	desc := newDescriptor("getMaxTransactionsPerBlock", smartcontract.IntegerType)
	md := newMethodAndPrice(p.getMaxTransactionsPerBlock, 1000000, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("getMaxBlockSize", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getMaxBlockSize, 1000000, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("getFeePerByte", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getFeePerByte, 1000000, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("isBlocked", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.isBlocked, 1000000, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("getMaxBlockSystemFee", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getMaxBlockSystemFee, 1000000, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("getExecFeeFactor", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getExecFeeFactor, 1000000, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setExecFeeFactor", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setExecFeeFactor, 3000000, callflag.WriteStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("getStoragePrice", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getStoragePrice, 1000000, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setStoragePrice", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setStoragePrice, 3000000, callflag.WriteStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setMaxBlockSize", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxBlockSize, 3000000, callflag.WriteStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setMaxTransactionsPerBlock", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxTransactionsPerBlock, 3000000, callflag.WriteStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setFeePerByte", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setFeePerByte, 3000000, callflag.WriteStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setMaxBlockSystemFee", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxBlockSystemFee, 3000000, callflag.WriteStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("blockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.blockAccount, 3000000, callflag.WriteStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("unblockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.unblockAccount, 3000000, callflag.WriteStates)
	p.AddMethod(md, desc)

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
	p.execFeeFactor = defaultExecFeeFactor
	p.feePerByte = defaultFeePerByte
	p.maxBlockSystemFee = defaultMaxBlockSystemFee
	p.maxVerificationGas = defaultMaxVerificationGas
	p.storagePrice = StoragePrice
	p.blockedAccounts = make([]util.Uint160, 0)

	return nil
}

// OnPersist implements Contract interface.
func (p *Policy) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements Contract interface.
func (p *Policy) PostPersist(ic *interop.Context) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.isValid {
		return nil
	}

	p.maxTransactionsPerBlock = getUint32WithKey(p.ContractID, ic.DAO, maxTransactionsPerBlockKey, defaultMaxTransactionsPerBlock)
	p.maxBlockSize = getUint32WithKey(p.ContractID, ic.DAO, maxBlockSizeKey, defaultMaxBlockSize)
	p.execFeeFactor = getUint32WithKey(p.ContractID, ic.DAO, execFeeFactorKey, defaultExecFeeFactor)
	p.feePerByte = getInt64WithKey(p.ContractID, ic.DAO, feePerByteKey, defaultFeePerByte)
	p.maxBlockSystemFee = getInt64WithKey(p.ContractID, ic.DAO, maxBlockSystemFeeKey, defaultMaxBlockSystemFee)
	p.maxVerificationGas = defaultMaxVerificationGas
	p.storagePrice = getUint32WithKey(p.ContractID, ic.DAO, storagePriceKey, StoragePrice)

	p.blockedAccounts = make([]util.Uint160, 0)
	siMap, err := ic.DAO.GetStorageItemsWithPrefix(p.ContractID, []byte{blockedAccountPrefix})
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
	return getUint32WithKey(p.ContractID, dao, maxTransactionsPerBlockKey, defaultMaxTransactionsPerBlock)
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
	return getUint32WithKey(p.ContractID, dao, maxBlockSizeKey, defaultMaxBlockSize)
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
	return getInt64WithKey(p.ContractID, dao, feePerByteKey, defaultFeePerByte)
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
	return getInt64WithKey(p.ContractID, dao, maxBlockSystemFeeKey, defaultMaxBlockSystemFee)
}

func (p *Policy) getExecFeeFactor(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetExecFeeFactorInternal(ic.DAO))))
}

// GetExecFeeFactorInternal returns current execution fee factor.
func (p *Policy) GetExecFeeFactorInternal(d dao.DAO) int64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return int64(p.execFeeFactor)
	}
	return int64(getUint32WithKey(p.ContractID, d, execFeeFactorKey, defaultExecFeeFactor))
}

func (p *Policy) setExecFeeFactor(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	if value <= 0 || maxExecFeeFactor < value {
		panic(fmt.Errorf("ExecFeeFactor must be between 0 and %d", maxExecFeeFactor))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err := setUint32WithKey(p.ContractID, ic.DAO, execFeeFactorKey, uint32(value))
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.Null{}
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

func (p *Policy) getStoragePrice(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetStoragePriceInternal(ic.DAO)))
}

// GetStoragePriceInternal returns current execution fee factor.
func (p *Policy) GetStoragePriceInternal(d dao.DAO) int64 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.isValid {
		return int64(p.storagePrice)
	}
	return int64(getUint32WithKey(p.ContractID, d, storagePriceKey, StoragePrice))
}

func (p *Policy) setStoragePrice(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	if value <= 0 || maxStoragePrice < value {
		panic(fmt.Errorf("StoragePrice must be between 0 and %d", maxStoragePrice))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err := setUint32WithKey(p.ContractID, ic.DAO, storagePriceKey, uint32(value))
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.Null{}
}

// setMaxTransactionsPerBlock is Policy contract method and  sets the upper limit
// of transactions per block.
func (p *Policy) setMaxTransactionsPerBlock(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := uint32(toBigInt(args[0]).Int64())
	if value > block.MaxTransactionsPerBlock {
		panic(fmt.Errorf("MaxTransactionsPerBlock cannot exceed the maximum allowed transactions per block = %d", block.MaxTransactionsPerBlock))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err := setUint32WithKey(p.ContractID, ic.DAO, maxTransactionsPerBlockKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.Null{}
}

// setMaxBlockSize is Policy contract method and sets maximum block size.
func (p *Policy) setMaxBlockSize(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := uint32(toBigInt(args[0]).Int64())
	if value > payload.MaxSize {
		panic(fmt.Errorf("MaxBlockSize cannot be more than the maximum payload size = %d", payload.MaxSize))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err := setUint32WithKey(p.ContractID, ic.DAO, maxBlockSizeKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.Null{}
}

// setFeePerByte is Policy contract method and sets transaction's fee per byte.
func (p *Policy) setFeePerByte(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toBigInt(args[0]).Int64()
	if value < 0 || value > maxFeePerByte {
		panic(fmt.Errorf("FeePerByte shouldn't be negative or greater than %d", maxFeePerByte))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err := setInt64WithKey(p.ContractID, ic.DAO, feePerByteKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.Null{}
}

// setMaxBlockSystemFee is Policy contract method and sets the maximum system fee per block.
func (p *Policy) setMaxBlockSystemFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toBigInt(args[0]).Int64()
	if value <= minBlockSystemFee {
		panic(fmt.Errorf("MaxBlockSystemFee cannot be less then %d", minBlockSystemFee))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	err := setInt64WithKey(p.ContractID, ic.DAO, maxBlockSystemFeeKey, value)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.Null{}
}

// blockAccount is Policy contract method and adds given account hash to the list
// of blocked accounts.
func (p *Policy) blockAccount(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	hash := toUint160(args[0])
	if p.IsBlockedInternal(ic.DAO, hash) {
		return stackitem.NewBool(false)
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	p.lock.Lock()
	defer p.lock.Unlock()
	err := ic.DAO.PutStorageItem(p.ContractID, key, &state.StorageItem{
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
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	hash := toUint160(args[0])
	if !p.IsBlockedInternal(ic.DAO, hash) {
		return stackitem.NewBool(false)
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	p.lock.Lock()
	defer p.lock.Unlock()
	err := ic.DAO.DeleteStorageItem(p.ContractID, key)
	if err != nil {
		panic(err)
	}
	p.isValid = false
	return stackitem.NewBool(true)
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
