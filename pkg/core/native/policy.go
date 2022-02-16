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
	NEO  *NEO
	lock sync.RWMutex
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

var _ interop.Contract = (*Policy)(nil)

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

	p.isValid = true
	p.execFeeFactor = defaultExecFeeFactor
	p.feePerByte = defaultFeePerByte
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

	p.execFeeFactor = uint32(getIntWithKey(p.ID, ic.DAO, execFeeFactorKey))
	p.feePerByte = getIntWithKey(p.ID, ic.DAO, feePerByteKey)
	p.maxVerificationGas = defaultMaxVerificationGas
	p.storagePrice = uint32(getIntWithKey(p.ID, ic.DAO, storagePriceKey))

	p.blockedAccounts = make([]util.Uint160, 0)
	siArr, err := ic.DAO.GetStorageItemsWithPrefix(p.ID, []byte{blockedAccountPrefix})
	if err != nil {
		return fmt.Errorf("failed to get blocked accounts from storage: %w", err)
	}
	for _, kv := range siArr {
		hash, err := util.Uint160DecodeBytesBE([]byte(kv.Key))
		if err != nil {
			return fmt.Errorf("failed to decode blocked account hash: %w", err)
		}
		p.blockedAccounts = append(p.blockedAccounts, hash)
	}
	// blockedAccounts should be sorted by account BE bytes, but GetStorageItemsWithPrefix
	// returns values sorted by key (which is account's BE bytes), so don't need to sort
	// one more time.

	p.isValid = true
	return nil
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
	setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, int64(value))
	p.isValid = false
	return stackitem.Null{}
}

// isBlocked is Policy contract method and checks whether provided account is blocked.
func (p *Policy) isBlocked(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	hash := toUint160(args[0])
	return stackitem.NewBool(p.IsBlockedInternal(ic.DAO, hash))
}

// IsBlockedInternal checks whether provided account is blocked.
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
	setIntWithKey(p.ID, ic.DAO, storagePriceKey, int64(value))
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
	setIntWithKey(p.ID, ic.DAO, feePerByteKey, value)
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
	for i := range ic.Natives {
		if ic.Natives[i].Metadata().Hash == hash {
			panic("cannot block native contract")
		}
	}
	if p.IsBlockedInternal(ic.DAO, hash) {
		return stackitem.NewBool(false)
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	p.lock.Lock()
	defer p.lock.Unlock()
	ic.DAO.PutStorageItem(p.ID, key, state.StorageItem{})
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
	ic.DAO.DeleteStorageItem(p.ID, key)
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
	return nil
}
