package native

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
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
	// defaultAttributeFee is a default fee for a transaction attribute those price wasn't set yet.
	defaultAttributeFee = 0
	// defaultNotaryAssistedFee is a default fee for a NotaryAssisted transaction attribute per key.
	defaultNotaryAssistedFee = 1000_0000 // 0.1 GAS
	// DefaultStoragePrice is the price to pay for 1 byte of storage.
	DefaultStoragePrice = 100000

	// maxExecFeeFactor is the maximum allowed execution fee factor.
	maxExecFeeFactor = 100
	// maxFeePerByte is the maximum allowed fee per byte value.
	maxFeePerByte = 100_000_000
	// maxStoragePrice is the maximum allowed price for a byte of storage.
	maxStoragePrice = 10000000
	// maxAttributeFee is the maximum allowed value for a transaction attribute fee.
	maxAttributeFee = 10_00000000

	// blockedAccountPrefix is a prefix used to store blocked account.
	blockedAccountPrefix = 15
	// attributeFeePrefix is a prefix used to store attribute fee.
	attributeFeePrefix = 20
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

	// p2pSigExtensionsEnabled defines whether the P2P signature extensions logic is relevant.
	p2pSigExtensionsEnabled bool
}

type PolicyCache struct {
	execFeeFactor      uint32
	feePerByte         int64
	maxVerificationGas int64
	storagePrice       uint32
	attributeFee       map[transaction.AttrType]uint32
	blockedAccounts    []util.Uint160
}

var (
	_ interop.Contract        = (*Policy)(nil)
	_ dao.NativeContractCache = (*PolicyCache)(nil)
)

// Copy implements NativeContractCache interface.
func (c *PolicyCache) Copy() dao.NativeContractCache {
	cp := &PolicyCache{}
	copyPolicyCache(c, cp)
	return cp
}

func copyPolicyCache(src, dst *PolicyCache) {
	*dst = *src
	dst.attributeFee = make(map[transaction.AttrType]uint32, len(src.attributeFee))
	for t, v := range src.attributeFee {
		dst.attributeFee[t] = v
	}
	dst.blockedAccounts = make([]util.Uint160, len(src.blockedAccounts))
	copy(dst.blockedAccounts, src.blockedAccounts)
}

// newPolicy returns Policy native contract.
func newPolicy(p2pSigExtensionsEnabled bool) *Policy {
	p := &Policy{
		ContractMD:              *interop.NewContractMD(nativenames.Policy, policyContractID),
		p2pSigExtensionsEnabled: p2pSigExtensionsEnabled,
	}
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

	desc = newDescriptor("getAttributeFee", smartcontract.IntegerType,
		manifest.NewParameter("attributeType", smartcontract.IntegerType))
	md = newMethodAndPrice(p.getAttributeFee, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = newDescriptor("setAttributeFee", smartcontract.VoidType,
		manifest.NewParameter("attributeType", smartcontract.IntegerType),
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(p.setAttributeFee, 1<<15, callflag.States)
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

// Metadata implements the Contract interface.
func (p *Policy) Metadata() *interop.ContractMD {
	return &p.ContractMD
}

// Initialize initializes Policy native contract and implements the Contract interface.
func (p *Policy) Initialize(ic *interop.Context) error {
	setIntWithKey(p.ID, ic.DAO, feePerByteKey, defaultFeePerByte)
	setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, defaultExecFeeFactor)
	setIntWithKey(p.ID, ic.DAO, storagePriceKey, DefaultStoragePrice)

	cache := &PolicyCache{
		execFeeFactor:      defaultExecFeeFactor,
		feePerByte:         defaultFeePerByte,
		maxVerificationGas: defaultMaxVerificationGas,
		storagePrice:       DefaultStoragePrice,
		attributeFee:       map[transaction.AttrType]uint32{},
		blockedAccounts:    make([]util.Uint160, 0),
	}
	if p.p2pSigExtensionsEnabled {
		setIntWithKey(p.ID, ic.DAO, []byte{attributeFeePrefix, byte(transaction.NotaryAssistedT)}, defaultNotaryAssistedFee)
		cache.attributeFee[transaction.NotaryAssistedT] = defaultNotaryAssistedFee
	}
	ic.DAO.SetCache(p.ID, cache)

	return nil
}

func (p *Policy) InitializeCache(blockHeight uint32, d *dao.Simple) error {
	cache := &PolicyCache{}
	err := p.fillCacheFromDAO(cache, d)
	if err != nil {
		return err
	}
	d.SetCache(p.ID, cache)
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

	cache.attributeFee = make(map[transaction.AttrType]uint32)
	d.Seek(p.ID, storage.SeekRange{Prefix: []byte{attributeFeePrefix}}, func(k, v []byte) bool {
		if len(k) != 1 {
			fErr = fmt.Errorf("unexpected attribute type len %d (%s)", len(k), hex.EncodeToString(k))
			return false
		}
		t := transaction.AttrType(k[0])
		value := bigint.FromBytes(v)
		if value == nil {
			fErr = fmt.Errorf("unexpected attribute value format: key=%s, value=%s", hex.EncodeToString(k), hex.EncodeToString(v))
			return false
		}
		cache.attributeFee[t] = uint32(value.Int64())
		return true
	})
	if fErr != nil {
		return fmt.Errorf("failed to initialize attribute fees: %w", fErr)
	}
	return nil
}

// OnPersist implements Contract interface.
func (p *Policy) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements Contract interface.
func (p *Policy) PostPersist(ic *interop.Context) error {
	return nil
}

// ActiveIn implements the Contract interface.
func (p *Policy) ActiveIn() *config.Hardfork {
	return nil
}

// getFeePerByte is a Policy contract method that returns the required transaction's fee
// per byte.
func (p *Policy) getFeePerByte(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetFeePerByteInternal(ic.DAO)))
}

// GetFeePerByteInternal returns required transaction's fee per byte.
func (p *Policy) GetFeePerByteInternal(dao *dao.Simple) int64 {
	cache := dao.GetROCache(p.ID).(*PolicyCache)
	return cache.feePerByte
}

// GetMaxVerificationGas returns the maximum gas allowed to be burned during verification.
func (p *Policy) GetMaxVerificationGas(dao *dao.Simple) int64 {
	cache := dao.GetROCache(p.ID).(*PolicyCache)
	return cache.maxVerificationGas
}

func (p *Policy) getExecFeeFactor(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetExecFeeFactorInternal(ic.DAO))))
}

// GetExecFeeFactorInternal returns current execution fee factor.
func (p *Policy) GetExecFeeFactorInternal(d *dao.Simple) int64 {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	return int64(cache.execFeeFactor)
}

func (p *Policy) setExecFeeFactor(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	if value <= 0 || maxExecFeeFactor < value {
		panic(fmt.Errorf("ExecFeeFactor must be between 1 and %d", maxExecFeeFactor))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, int64(value))
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	cache.execFeeFactor = value
	return stackitem.Null{}
}

// isBlocked is Policy contract method that checks whether provided account is blocked.
func (p *Policy) isBlocked(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	hash := toUint160(args[0])
	_, blocked := p.isBlockedInternal(ic.DAO.GetROCache(p.ID).(*PolicyCache), hash)
	return stackitem.NewBool(blocked)
}

// IsBlocked checks whether provided account is blocked. Normally it uses Policy
// cache, falling back to the DB queries when Policy cache is not available yet
// (the only case is native cache initialization pipeline, where native Neo cache
// is being initialized before the Policy's one).
func (p *Policy) IsBlocked(dao *dao.Simple, hash util.Uint160) bool {
	cache := dao.GetROCache(p.ID)
	if cache == nil {
		key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
		return dao.GetStorageItem(p.ID, key) == nil
	}
	_, isBlocked := p.isBlockedInternal(cache.(*PolicyCache), hash)
	return isBlocked
}

// isBlockedInternal checks whether provided account is blocked. It returns position
// of the blocked account in the blocked accounts list (or the position it should be
// put at).
func (p *Policy) isBlockedInternal(roCache *PolicyCache, hash util.Uint160) (int, bool) {
	length := len(roCache.blockedAccounts)
	i := sort.Search(length, func(i int) bool {
		return !roCache.blockedAccounts[i].Less(hash)
	})
	if length != 0 && i != length && roCache.blockedAccounts[i].Equals(hash) {
		return i, true
	}
	return i, false
}

func (p *Policy) getStoragePrice(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetStoragePriceInternal(ic.DAO)))
}

// GetStoragePriceInternal returns current execution fee factor.
func (p *Policy) GetStoragePriceInternal(d *dao.Simple) int64 {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	return int64(cache.storagePrice)
}

func (p *Policy) setStoragePrice(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	if value <= 0 || maxStoragePrice < value {
		panic(fmt.Errorf("StoragePrice must be between 1 and %d", maxStoragePrice))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	setIntWithKey(p.ID, ic.DAO, storagePriceKey, int64(value))
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	cache.storagePrice = value
	return stackitem.Null{}
}

func (p *Policy) getAttributeFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	t := transaction.AttrType(toUint8(args[0]))
	if !transaction.IsValidAttrType(ic.Chain.GetConfig().ReservedAttributes, t) {
		panic(fmt.Errorf("invalid attribute type: %d", t))
	}
	return stackitem.NewBigInteger(big.NewInt(p.GetAttributeFeeInternal(ic.DAO, t)))
}

// GetAttributeFeeInternal returns required transaction's attribute fee.
func (p *Policy) GetAttributeFeeInternal(d *dao.Simple, t transaction.AttrType) int64 {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	v, ok := cache.attributeFee[t]
	if !ok {
		// We may safely omit this part, but let it be here in case if defaultAttributeFee value is changed.
		v = defaultAttributeFee
	}
	return int64(v)
}

func (p *Policy) setAttributeFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	t := transaction.AttrType(toUint8(args[0]))
	value := toUint32(args[1])
	if !transaction.IsValidAttrType(ic.Chain.GetConfig().ReservedAttributes, t) {
		panic(fmt.Errorf("invalid attribute type: %d", t))
	}
	if value > maxAttributeFee {
		panic(fmt.Errorf("attribute value is out of range: %d", value))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	setIntWithKey(p.ID, ic.DAO, []byte{attributeFeePrefix, byte(t)}, int64(value))
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	cache.attributeFee[t] = value
	return stackitem.Null{}
}

// setFeePerByte is a Policy contract method that sets transaction's fee per byte.
func (p *Policy) setFeePerByte(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toBigInt(args[0]).Int64()
	if value < 0 || value > maxFeePerByte {
		panic(fmt.Errorf("FeePerByte shouldn't be negative or greater than %d", maxFeePerByte))
	}
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	setIntWithKey(p.ID, ic.DAO, feePerByteKey, value)
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	cache.feePerByte = value
	return stackitem.Null{}
}

// blockAccount is a Policy contract method that adds the given account hash to the list
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
	return stackitem.NewBool(p.blockAccountInternal(ic.DAO, hash))
}
func (p *Policy) blockAccountInternal(d *dao.Simple, hash util.Uint160) bool {
	i, blocked := p.isBlockedInternal(d.GetROCache(p.ID).(*PolicyCache), hash)
	if blocked {
		return false
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	d.PutStorageItem(p.ID, key, state.StorageItem{})
	cache := d.GetRWCache(p.ID).(*PolicyCache)
	if len(cache.blockedAccounts) == i {
		cache.blockedAccounts = append(cache.blockedAccounts, hash)
	} else {
		cache.blockedAccounts = append(cache.blockedAccounts[:i+1], cache.blockedAccounts[i:]...)
		cache.blockedAccounts[i] = hash
	}
	return true
}

// unblockAccount is a Policy contract method that removes the given account hash from
// the list of blocked accounts.
func (p *Policy) unblockAccount(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	if !p.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	hash := toUint160(args[0])
	i, blocked := p.isBlockedInternal(ic.DAO.GetROCache(p.ID).(*PolicyCache), hash)
	if !blocked {
		return stackitem.NewBool(false)
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	ic.DAO.DeleteStorageItem(p.ID, key)
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	cache.blockedAccounts = append(cache.blockedAccounts[:i], cache.blockedAccounts[i+1:]...)
	return stackitem.NewBool(true)
}

// CheckPolicy checks whether a transaction conforms to the current policy restrictions,
// like not being signed by a blocked account or not exceeding the block-level system
// fee limit.
func (p *Policy) CheckPolicy(d *dao.Simple, tx *transaction.Transaction) error {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	for _, signer := range tx.Signers {
		if _, isBlocked := p.isBlockedInternal(cache, signer.Account); isBlocked {
			return fmt.Errorf("account %s is blocked", signer.Account.StringLE())
		}
	}
	return nil
}
