package native

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	policyContractID = -7

	defaultExecFeeFactor      = interop.DefaultBaseExecFee
	defaultFeePerByte         = 1000
	defaultMaxVerificationGas = 1_50000000
	// DefaultStoragePrice is the price to pay for 1 byte of storage.
	DefaultStoragePrice = 100000

	// maxExecFeeFactor is the maximum allowed execution fee factor.
	maxExecFeeFactor = 100
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
	// storagePriceKey is a key used to store storage price.
	storagePriceKey = []byte{19}
)

// Policy represents Policy native contract.
type Policy struct {
	interop.ContractMD
	NEO *NEO
}

type PolicyCache struct {
	// isValid defies whether cached values were changed during the current
	// consensus iteration. If false, these values will be updated after
	// blockchain DAO persisting. If true, we can safely use cached values.
	isValid            bool
	execFeeFactor      uint32
	feePerByte         int64
	maxVerificationGas int64
	storagePrice       uint32
	blockedAccounts    []util.Uint160
}

var (
	_ interop.Contract            = (*Policy)(nil)
	_ storage.NativeContractCache = (*PolicyCache)(nil)
)

// Copy implements NativeContractCache interface.
func (c *PolicyCache) Copy() storage.NativeContractCache {
	cp := &PolicyCache{}
	copyPolicyCache(c, cp)
	return cp
}

// Persist implements NativeContractCache interface.
func (c *PolicyCache) Persist(ps storage.NativeContractCache) (storage.NativeContractCache, error) {
	if ps == nil {
		ps = &PolicyCache{}
	}
	psCache, ok := ps.(*PolicyCache)
	if !ok {
		return nil, errors.New("not a Policy native cache")
	}
	copyPolicyCache(c, psCache)
	return psCache, nil
}

func copyPolicyCache(src, dst *PolicyCache) {
	*dst = *src
	dst.blockedAccounts = make([]util.Uint160, len(src.blockedAccounts))
	copy(dst.blockedAccounts, src.blockedAccounts)
}

// newPolicy returns Policy native contract.
func newPolicy() *Policy {
	p := &Policy{ContractMD: *interop.NewContractMD(nativenames.Policy, policyContractID)}
	defer p.UpdateHash()

	desc := newDescriptor("getFeePerByte", smartcontract.IntegerType)
	md := newMethodAndPrice(p.getFeePerByte, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("isBlocked", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.isBlocked, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("getExecFeeFactor", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getExecFeeFactor, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setExecFeeFactor", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setExecFeeFactor, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("getStoragePrice", smartcontract.IntegerType)
	md = newMethodAndPrice(p.getStoragePrice, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setStoragePrice", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setStoragePrice, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("setFeePerByte", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setFeePerByte, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("blockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.blockAccount, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	desc = newDescriptor("unblockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(p.unblockAccount, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	return p
}

// Metadata implements Contract interface.
func (p *Policy) Metadata() *interop.ContractMD {
	return &p.ContractMD
}

// Initialize initializes Policy native contract and implements Contract interface.
func (p *Policy) Initialize(ic *interop.Context) error {
	setIntWithKey(p.ID, ic.DAO, feePerByteKey, defaultFeePerByte)
	setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, defaultExecFeeFactor)
	setIntWithKey(p.ID, ic.DAO, storagePriceKey, DefaultStoragePrice)

	cache := &PolicyCache{}
	cache.isValid = true
	cache.execFeeFactor = defaultExecFeeFactor
	cache.feePerByte = defaultFeePerByte
	cache.maxVerificationGas = defaultMaxVerificationGas
	cache.storagePrice = DefaultStoragePrice
	cache.blockedAccounts = make([]util.Uint160, 0)
	ic.DAO.Store.SetCache(p.ID, cache)

	return nil
}

func (p *Policy) InitializeCache(d *dao.Simple) error {
	cache := &PolicyCache{}
	err := p.fillCacheFromDAO(cache, d)
	if err != nil {
		return err
	}
	d.Store.SetCache(p.ID, cache)
	return nil
}

func (p *Policy) fillCacheFromDAO(cache *PolicyCache, d *dao.Simple) error {
	cache.execFeeFactor = uint32(getIntWithKey(p.ID, d, execFeeFactorKey))
	cache.feePerByte = getIntWithKey(p.ID, d, feePerByteKey)
	cache.maxVerificationGas = defaultMaxVerificationGas
	cache.storagePrice = uint32(getIntWithKey(p.ID, d, storagePriceKey))

	cache.blockedAccounts = make([]util.Uint160, 0)
	var fErr error
	d.Seek(p.ID, storage.SeekRange{Prefix: []byte{blockedAccountPrefix}}, func(k, _ []byte) bool {
		hash, err := util.Uint160DecodeBytesBE(k)
		if err != nil {
			fErr = fmt.Errorf("failed to decode blocked account hash: %w", err)
			return false
		}
		cache.blockedAccounts = append(cache.blockedAccounts, hash)
		return true
	})
	if fErr != nil {
		return fmt.Errorf("failed to initialize blocked accounts: %w", fErr)
	}
	cache.isValid = true
	return nil
}

// OnPersist implements Contract interface.
func (p *Policy) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements Contract interface.
func (p *Policy) PostPersist(ic *interop.Context) error {
	cache := ic.DAO.Store.GetRWCache(p.ID).(*PolicyCache)
	if cache.isValid {
		return nil
	}

	return p.fillCacheFromDAO(cache, ic.DAO)
}

// getFeePerByte is Policy contract method and returns required transaction's fee
// per byte.
func (p *Policy) getFeePerByte(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetFeePerByteInternal(ic.DAO)))
}

// GetFeePerByteInternal returns required transaction's fee per byte.
func (p *Policy) GetFeePerByteInternal(dao *dao.Simple) int64 {
	cache := dao.Store.GetROCache(p.ID).(*PolicyCache)
	if cache.isValid {
		return cache.feePerByte
	}
	return getIntWithKey(p.ID, dao, feePerByteKey)
}

// GetMaxVerificationGas returns maximum gas allowed to be burned during verificaion.
func (p *Policy) GetMaxVerificationGas(dao *dao.Simple) int64 {
	cache := dao.Store.GetROCache(p.ID).(*PolicyCache)
	if cache.isValid {
		return cache.maxVerificationGas
	}
	return defaultMaxVerificationGas
}

func (p *Policy) getExecFeeFactor(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetExecFeeFactorInternal(ic.DAO))))
}

// GetExecFeeFactorInternal returns current execution fee factor.
func (p *Policy) GetExecFeeFactorInternal(d *dao.Simple) int64 {
	cache := d.Store.GetROCache(p.ID).(*PolicyCache)
	if cache.isValid {
		return int64(cache.execFeeFactor)
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
	cache := ic.DAO.Store.GetRWCache(p.ID).(*PolicyCache)
	setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, int64(value))
	cache.isValid = false
	return stackitem.Null{}
}

// isBlocked is Policy contract method and checks whether provided account is blocked.
func (p *Policy) isBlocked(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	hash := toUint160(args[0])
	return stackitem.NewBool(p.IsBlockedInternal(ic.DAO, hash))
}

// IsBlockedInternal checks whether provided account is blocked.
func (p *Policy) IsBlockedInternal(dao *dao.Simple, hash util.Uint160) bool {
	cache := dao.Store.GetROCache(p.ID).(*PolicyCache)
	if cache.isValid {
		length := len(cache.blockedAccounts)
		i := sort.Search(length, func(i int) bool {
			return !cache.blockedAccounts[i].Less(hash)
		})
		if length != 0 && i != length && cache.blockedAccounts[i].Equals(hash) {
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
func (p *Policy) GetStoragePriceInternal(d *dao.Simple) int64 {
	cache := d.Store.GetROCache(p.ID).(*PolicyCache)
	if cache.isValid {
		return int64(cache.storagePrice)
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
	cache := ic.DAO.Store.GetRWCache(p.ID).(*PolicyCache)
	setIntWithKey(p.ID, ic.DAO, storagePriceKey, int64(value))
	cache.isValid = false
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
	cache := ic.DAO.Store.GetRWCache(p.ID).(*PolicyCache)
	setIntWithKey(p.ID, ic.DAO, feePerByteKey, value)
	cache.isValid = false
	return stackitem.Null{}
}

// blockAccount is Policy contract method and adds given account hash to the list
// of blocked accounts.
func (p *Policy) blockAccount(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	hash := toUint160(args[0])
	for i := range ic.Natives {
		if ic.Natives[i].Metadata().Hash == hash {
			panic("cannot block native contract")
		}
	}
	if p.IsBlockedInternal(ic.DAO, hash) {
		return stackitem.NewBool(false)
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	cache := ic.DAO.Store.GetRWCache(p.ID).(*PolicyCache)
	ic.DAO.PutStorageItem(p.ID, key, state.StorageItem{})
	cache.isValid = false
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
	cache := ic.DAO.Store.GetRWCache(p.ID).(*PolicyCache)
	ic.DAO.DeleteStorageItem(p.ID, key)
	cache.isValid = false
	return stackitem.NewBool(true)
}

// CheckPolicy checks whether transaction conforms to current policy restrictions
// like not being signed by blocked account or not exceeding block-level system
// fee limit.
func (p *Policy) CheckPolicy(d *dao.Simple, tx *transaction.Transaction) error {
	for _, signer := range tx.Signers {
		if p.IsBlockedInternal(d, signer.Account) {
			return fmt.Errorf("account %s is blocked", signer.Account.StringLE())
		}
	}
	return nil
}
