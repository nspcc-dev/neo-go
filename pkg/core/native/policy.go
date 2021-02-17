package native

import (
	"fmt"
	"math/big"
	"sort"
	"sync"

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
	policyContractID = -5

	defaultMaxBlockSize       = 1024 * 256
	defaultExecFeeFactor      = interop.DefaultBaseExecFee
	defaultFeePerByte         = 1000
	defaultMaxVerificationGas = 50000000
	defaultMaxBlockSystemFee  = 9000 * GASFactor
	// DefaultStoragePrice is the price to pay for 1 byte of storage.
	DefaultStoragePrice = 100000

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
	isValid            bool
	maxBlockSize       uint32
	execFeeFactor      uint32
	feePerByte         int64
	maxBlockSystemFee  int64
	maxVerificationGas int64
	storagePrice       uint32
	blockedAccounts    []util.Uint160
}

var _ interop.Contract = (*Policy)(nil)

// newPolicy returns Policy native contract.
func newPolicy() *Policy {
	p := &Policy{ContractMD: *interop.NewContractMD(nativenames.Policy, policyContractID)}
	defer p.UpdateHash()

	desc := newDescriptor("getMaxBlockSize", smartcontract.IntegerType)
	md := newMethodAndPrice(p.getMaxBlockSize, 1000000, callflag.ReadStates)
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
	md = newMethodAndPrice(p.setExecFeeFactor, 3000000, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("getStoragePrice", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getStoragePrice, 1000000, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setStoragePrice", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setStoragePrice, 3000000, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("setMaxBlockSize", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxBlockSize, 3000000, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("setFeePerByte", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setFeePerByte, 3000000, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("setMaxBlockSystemFee", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setMaxBlockSystemFee, 3000000, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("blockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.blockAccount, 3000000, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("unblockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.unblockAccount, 3000000, callflag.States)
	p.AddMethod(md, desc)

	return p
}

// Metadata implements Contract interface.
func (p *Policy) Metadata() *interop.ContractMD {
	return &p.ContractMD
}

// Initialize initializes Policy native contract and implements Contract interface.
func (p *Policy) Initialize(ic *interop.Context) error {
	if err := setIntWithKey(p.ID, ic.DAO, feePerByteKey, defaultFeePerByte); err != nil {
		return err
	}
	if err := setIntWithKey(p.ID, ic.DAO, maxBlockSizeKey, defaultMaxBlockSize); err != nil {
		return err
	}
	if err := setIntWithKey(p.ID, ic.DAO, maxBlockSystemFeeKey, defaultMaxBlockSystemFee); err != nil {
		return err
	}
	if err := setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, defaultExecFeeFactor); err != nil {
		return err
	}
	if err := setIntWithKey(p.ID, ic.DAO, storagePriceKey, DefaultStoragePrice); err != nil {
		return err
	}

	p.isValid = true
	p.maxBlockSize = defaultMaxBlockSize
	p.execFeeFactor = defaultExecFeeFactor
	p.feePerByte = defaultFeePerByte
	p.maxBlockSystemFee = defaultMaxBlockSystemFee
	p.maxVerificationGas = defaultMaxVerificationGas
	p.storagePrice = DefaultStoragePrice
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

	p.maxBlockSize = uint32(getIntWithKey(p.ID, ic.DAO, maxBlockSizeKey))
	p.execFeeFactor = uint32(getIntWithKey(p.ID, ic.DAO, execFeeFactorKey))
	p.feePerByte = getIntWithKey(p.ID, ic.DAO, feePerByteKey)
	p.maxBlockSystemFee = getIntWithKey(p.ID, ic.DAO, maxBlockSystemFeeKey)
	p.maxVerificationGas = defaultMaxVerificationGas
	p.storagePrice = uint32(getIntWithKey(p.ID, ic.DAO, storagePriceKey))

	p.blockedAccounts = make([]util.Uint160, 0)
	siMap, err := ic.DAO.GetStorageItemsWithPrefix(p.ID, []byte{blockedAccountPrefix})
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
	return uint32(getIntWithKey(p.ID, dao, maxBlockSizeKey))
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
	return getIntWithKey(p.ID, dao, feePerByteKey)
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
	return getIntWithKey(p.ID, dao, maxBlockSystemFeeKey)
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
	return getIntWithKey(p.ID, d, execFeeFactorKey)
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
	err := setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, int64(value))
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
	return dao.GetStorageItem(p.ID, key) != nil
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
	return getIntWithKey(p.ID, d, storagePriceKey)
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
	err := setIntWithKey(p.ID, ic.DAO, storagePriceKey, int64(value))
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
	err := setIntWithKey(p.ID, ic.DAO, maxBlockSizeKey, int64(value))
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
	err := setIntWithKey(p.ID, ic.DAO, feePerByteKey, value)
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
	err := setIntWithKey(p.ID, ic.DAO, maxBlockSystemFeeKey, value)
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
	err := ic.DAO.PutStorageItem(p.ID, key, &state.StorageItem{
		Value: []byte{},
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
	err := ic.DAO.DeleteStorageItem(p.ID, key)
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
