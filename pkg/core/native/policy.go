package native

import (
	"cmp"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"maps"
	"math/big"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
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
	maxExecFeeFactor uint32 = 100
	// maxFeePerByte is the maximum allowed fee per byte value.
	maxFeePerByte = 100_000_000
	// maxStoragePrice is the maximum allowed price for a byte of storage.
	maxStoragePrice = 10000000
	// maxAttributeFee is the maximum allowed value for a transaction attribute fee.
	maxAttributeFee = 10_00000000
	// maxMillisecondsPerBlock is the maximum allowed value (in milliseconds) for a block generation time.
	maxMillisecondsPerBlock = 30_000
	// maxMaxVUBIncrement the maximum value for upper increment size of blockchain
	// height (in blocks) exceeding that a transaction should fail validation.
	maxMaxVUBIncrement = 86400
	// the maximum value of maximum number of blocks reachable to contracts.
	maxMaxTraceableBlocks = 2102400

	// blockedAccountPrefix is a prefix used to store blocked account.
	blockedAccountPrefix = 15
	// whitelistedFeeContractPrefix is a prefix used to store whitelisted contract.
	whitelistedFeeContractPrefix = 16
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
	// msPerBlockKey is a key used to store block generation time.
	msPerBlockKey = []byte{21}
	// maxVUBIncrementKey is a key used to store maximum ValidUntilBlock increment.
	maxVUBIncrementKey = []byte{22}
	// MaxTraceableBlocksKey is a key used to store the maximum number of traceable blocks.
	MaxTraceableBlocksKey = []byte{23}
)

// Policy represents Policy native contract.
type Policy struct {
	interop.ContractMD
	NEO INEO
}

type PolicyCache struct {
	execFeeFactor        uint32
	faunInitialized      bool
	feePerByte           int64
	maxVerificationGas   int64
	storagePrice         uint32
	msPerBlock           uint32
	maxVUBIncrement      uint32
	maxTraceableBlocks   uint32
	attributeFee         map[transaction.AttrType]uint32
	blockedAccounts      []util.Uint160
	whitelistedContracts []whitelistedContract
}

// whitelistedContract is a structure representing a whitelisted contract cache
// entry.
type whitelistedContract struct {
	state.WhitelistFeeContract
	offset uint32
}

// Compare compares two whitelistedContract structures in the order that matches
// the serialization format of a key used to store a whitelisted contract record.
func (c whitelistedContract) Compare(other whitelistedContract) int {
	return cmp.Or(
		c.Hash.Compare(other.Hash),
		cmp.Compare(c.offset, other.offset), // offset is stored in BE.
	)
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
	dst.attributeFee = maps.Clone(src.attributeFee)
	dst.blockedAccounts = slices.Clone(src.blockedAccounts)
	dst.whitelistedContracts = slices.Clone(src.whitelistedContracts)
}

// newPolicy returns Policy native contract.
func newPolicy() *Policy {
	p := &Policy{ContractMD: *interop.NewContractMD(nativenames.Policy, nativeids.PolicyContract)}
	defer p.BuildHFSpecificMD(p.ActiveIn())

	desc := NewDescriptor("getFeePerByte", smartcontract.IntegerType)
	md := NewMethodAndPrice(p.getFeePerByte, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = NewDescriptor("isBlocked", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = NewMethodAndPrice(p.isBlocked, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getExecFeeFactor", smartcontract.IntegerType)
	md = NewMethodAndPrice(p.getExecFeeFactor, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setExecFeeFactor", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setExecFeeFactor, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getStoragePrice", smartcontract.IntegerType)
	md = NewMethodAndPrice(p.getStoragePrice, 1<<15, callflag.ReadStates)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setStoragePrice", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setStoragePrice, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getAttributeFee", smartcontract.IntegerType,
		manifest.NewParameter("attributeType", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.getAttributeFeeV0, 1<<15, callflag.ReadStates, config.HFDefault, transaction.NotaryAssistedActivation)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getAttributeFee", smartcontract.IntegerType,
		manifest.NewParameter("attributeType", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.getAttributeFeeV1, 1<<15, callflag.ReadStates, transaction.NotaryAssistedActivation)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setAttributeFee", smartcontract.VoidType,
		manifest.NewParameter("attributeType", smartcontract.IntegerType),
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setAttributeFeeV0, 1<<15, callflag.States, config.HFDefault, transaction.NotaryAssistedActivation)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setAttributeFee", smartcontract.VoidType,
		manifest.NewParameter("attributeType", smartcontract.IntegerType),
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setAttributeFeeV1, 1<<15, callflag.States, transaction.NotaryAssistedActivation)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setFeePerByte", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setFeePerByte, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	desc = NewDescriptor("blockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = NewMethodAndPrice(p.blockAccount, 1<<15, callflag.States, config.HFDefault, config.HFFaun)
	p.AddMethod(md, desc)

	desc = NewDescriptor("blockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = NewMethodAndPrice(p.blockAccount, 1<<15, callflag.States|callflag.AllowNotify, config.HFFaun)
	p.AddMethod(md, desc)

	desc = NewDescriptor("unblockAccount", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = NewMethodAndPrice(p.unblockAccount, 1<<15, callflag.States)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getMaxValidUntilBlockIncrement", smartcontract.IntegerType)
	md = NewMethodAndPrice(p.getMaxValidUntilBlockIncrement, 1<<15, callflag.ReadStates, config.HFEchidna)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setMaxValidUntilBlockIncrement", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setMaxValidUntilBlockIncrement, 1<<15, callflag.States, config.HFEchidna)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getMillisecondsPerBlock", smartcontract.IntegerType)
	md = NewMethodAndPrice(p.getMillisecondsPerBlock, 1<<15, callflag.ReadStates, config.HFEchidna)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setMillisecondsPerBlock", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setMillisecondsPerBlock, 1<<15, callflag.States|callflag.AllowNotify, config.HFEchidna)
	p.AddMethod(md, desc)

	eDesc := NewEventDescriptor("MillisecondsPerBlockChanged",
		manifest.NewParameter("old", smartcontract.IntegerType),
		manifest.NewParameter("new", smartcontract.IntegerType),
	)
	eMD := NewEvent(eDesc, config.HFEchidna)
	p.AddEvent(eMD)

	desc = NewDescriptor("getMaxTraceableBlocks", smartcontract.IntegerType)
	md = NewMethodAndPrice(p.getMaxTraceableBlocks, 1<<15, callflag.ReadStates, config.HFEchidna)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setMaxTraceableBlocks", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setMaxTraceableBlocks, 1<<15, callflag.States, config.HFEchidna)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getBlockedAccounts", smartcontract.InteropInterfaceType)
	md = NewMethodAndPrice(p.getBlockedAccounts, 1<<15, callflag.ReadStates, config.HFFaun)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getExecPicoFeeFactor", smartcontract.IntegerType)
	md = NewMethodAndPrice(p.getExecPicoFeeFactor, 1<<15, callflag.ReadStates, config.HFFaun)
	p.AddMethod(md, desc)

	desc = NewDescriptor("setWhitelistFeeContract", smartcontract.VoidType,
		manifest.NewParameter("contractHash", smartcontract.Hash160Type),
		manifest.NewParameter("method", smartcontract.StringType),
		manifest.NewParameter("argCount", smartcontract.IntegerType),
		manifest.NewParameter("fixedFee", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.setWhitelistFeeContract, 1<<15, callflag.States|callflag.AllowNotify, config.HFFaun)
	p.AddMethod(md, desc)

	desc = NewDescriptor("removeWhitelistFeeContract", smartcontract.VoidType,
		manifest.NewParameter("contractHash", smartcontract.Hash160Type),
		manifest.NewParameter("method", smartcontract.StringType),
		manifest.NewParameter("argCount", smartcontract.IntegerType))
	md = NewMethodAndPrice(p.removeWhitelistFeeContract, 1<<15, callflag.States|callflag.AllowNotify, config.HFFaun)
	p.AddMethod(md, desc)

	desc = NewDescriptor("getWhitelistFeeContracts", smartcontract.InteropInterfaceType)
	md = NewMethodAndPrice(p.getWhitelistFeeContracts, 1<<15, callflag.ReadStates, config.HFFaun)
	p.AddMethod(md, desc)

	eDesc = NewEventDescriptor("WhitelistFeeChanged",
		manifest.NewParameter("contract", smartcontract.Hash160Type),
		manifest.NewParameter("method", smartcontract.StringType),
		manifest.NewParameter("argCount", smartcontract.IntegerType),
		manifest.NewParameter("fee", smartcontract.AnyType),
	)
	eMD = NewEvent(eDesc, config.HFFaun)
	p.AddEvent(eMD)

	return p
}

// Metadata implements the Contract interface.
func (p *Policy) Metadata() *interop.ContractMD {
	return &p.ContractMD
}

// Initialize initializes Policy native contract and implements the Contract interface.
func (p *Policy) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	if hf == p.ActiveIn() {
		setIntWithKey(p.ID, ic.DAO, feePerByteKey, defaultFeePerByte)
		setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, defaultExecFeeFactor)
		setIntWithKey(p.ID, ic.DAO, storagePriceKey, DefaultStoragePrice)

		cache := &PolicyCache{
			execFeeFactor:        defaultExecFeeFactor,
			feePerByte:           defaultFeePerByte,
			maxVerificationGas:   defaultMaxVerificationGas,
			storagePrice:         DefaultStoragePrice,
			attributeFee:         map[transaction.AttrType]uint32{},
			blockedAccounts:      make([]util.Uint160, 0),
			whitelistedContracts: make([]whitelistedContract, 0),
		}
		ic.DAO.SetCache(p.ID, cache)
	}

	if hf != nil && *hf == config.HFEchidna {
		cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)

		maxVUBIncrement := ic.Chain.GetConfig().Genesis.MaxValidUntilBlockIncrement
		setIntWithKey(p.ID, ic.DAO, maxVUBIncrementKey, int64(maxVUBIncrement))
		cache.maxVUBIncrement = maxVUBIncrement

		msPerBlock := ic.Chain.GetConfig().Genesis.TimePerBlock.Milliseconds()
		setIntWithKey(p.ID, ic.DAO, msPerBlockKey, msPerBlock)
		cache.msPerBlock = uint32(msPerBlock)

		maxTraceableBlocks := ic.Chain.GetConfig().Genesis.MaxTraceableBlocks
		setIntWithKey(p.ID, ic.DAO, MaxTraceableBlocksKey, int64(maxTraceableBlocks))
		cache.maxTraceableBlocks = maxTraceableBlocks

		setIntWithKey(p.ID, ic.DAO, []byte{attributeFeePrefix, byte(transaction.NotaryAssistedT)}, defaultNotaryAssistedFee)
		cache.attributeFee[transaction.NotaryAssistedT] = defaultNotaryAssistedFee
	}

	if hf != nil && *hf == config.HFFaun {
		cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
		cache.execFeeFactor = cache.execFeeFactor * vm.ExecFeeFactorMultiplier
		cache.faunInitialized = true
		setIntWithKey(p.ID, ic.DAO, execFeeFactorKey, int64(cache.execFeeFactor))
	}

	return nil
}

func (p *Policy) InitializeCache(isHardforkEnabled interop.IsHardforkEnabled, blockHeight uint32, d *dao.Simple) error {
	cache := &PolicyCache{}
	err := p.fillCacheFromDAO(cache, d, isHardforkEnabled, blockHeight)
	if err != nil {
		return err
	}
	d.SetCache(p.ID, cache)
	return nil
}

func (p *Policy) fillCacheFromDAO(cache *PolicyCache, d *dao.Simple, isHardforkEnabled interop.IsHardforkEnabled, blockHeight uint32) error {
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

	var echidna = config.HFEchidna
	if isHardforkEnabled(&echidna, blockHeight) {
		cache.maxVUBIncrement = uint32(getIntWithKey(p.ID, d, maxVUBIncrementKey))
		cache.msPerBlock = uint32(getIntWithKey(p.ID, d, msPerBlockKey))
		cache.maxTraceableBlocks = uint32(getIntWithKey(p.ID, d, MaxTraceableBlocksKey))
	}

	cache.whitelistedContracts = make([]whitelistedContract, 0)
	var faun = config.HFFaun
	if isHardforkEnabled(&faun, blockHeight) {
		cache.faunInitialized = true
		d.Seek(p.ID, storage.SeekRange{Prefix: []byte{whitelistedFeeContractPrefix}}, func(k, v []byte) bool {
			if len(k) != util.Uint160Size+4 {
				fErr = fmt.Errorf("unexpected whitelisted contract key length %d vs %d", len(k), util.Uint160Size+4)
				return false
			}
			offset := binary.BigEndian.Uint32(k[util.Uint160Size:])

			c := new(state.WhitelistFeeContract)
			err := stackitem.DeserializeConvertible(v, c)
			if err != nil {
				fErr = fmt.Errorf("failed to deserialize whitelisted contract record: %w", err)
				return false
			}

			cache.whitelistedContracts = append(cache.whitelistedContracts, whitelistedContract{
				WhitelistFeeContract: *c,
				offset:               offset,
			})
			return true
		})
		if fErr != nil {
			return fmt.Errorf("failed to initialize whitelisted contracts: %w", fErr)
		}
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

// getExecFeeFactor returns the current execution fee factor in Datoshi units.
func (p *Policy) getExecFeeFactor(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetExecFeeFactorInternal(ic.DAO) / vm.ExecFeeFactorMultiplier))
}

// GetExecFeeFactorInternal returns the current execution fee factor in picoGAS
// units.
func (p *Policy) GetExecFeeFactorInternal(d *dao.Simple) int64 {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	if cache.faunInitialized {
		return int64(cache.execFeeFactor)
	}
	return int64(cache.execFeeFactor * vm.ExecFeeFactorMultiplier)
}

// getExecPicoFeeFactor is active starting from [config.HFFaun] and returns the
// current execution fee factor in picoGAS units.
func (p *Policy) getExecPicoFeeFactor(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	cache := ic.DAO.GetROCache(p.ID).(*PolicyCache)
	return stackitem.NewBigInteger(big.NewInt(int64(cache.execFeeFactor)))
}

func (p *Policy) setExecFeeFactor(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	maxValue := maxExecFeeFactor
	if ic.IsHardforkEnabled(config.HFFaun) {
		maxValue *= vm.ExecFeeFactorMultiplier
	}
	if value <= 0 || maxValue < value {
		panic(fmt.Errorf("ExecFeeFactor must be between 1 and %d", maxExecFeeFactor))
	}
	if !p.NEO.CheckCommittee(ic) {
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
		return dao.GetStorageItem(p.ID, key) != nil
	}
	_, isBlocked := p.isBlockedInternal(cache.(*PolicyCache), hash)
	return isBlocked
}

// isBlockedInternal checks whether provided account is blocked. It returns position
// of the blocked account in the blocked accounts list (or the position it should be
// put at).
func (p *Policy) isBlockedInternal(roCache *PolicyCache, hash util.Uint160) (int, bool) {
	return slices.BinarySearchFunc(roCache.blockedAccounts, hash, util.Uint160.Compare)
}

func (p *Policy) getStoragePrice(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(p.GetStoragePriceInternal(ic.DAO) / vm.ExecFeeFactorMultiplier))
}

// GetStoragePriceInternal returns the current storage price in picoGAS units.
func (p *Policy) GetStoragePriceInternal(d *dao.Simple) int64 {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	return int64(cache.storagePrice) * vm.ExecFeeFactorMultiplier
}

func (p *Policy) setStoragePrice(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	if value <= 0 || maxStoragePrice < value {
		panic(fmt.Errorf("StoragePrice must be between 1 and %d", maxStoragePrice))
	}
	if !p.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}
	setIntWithKey(p.ID, ic.DAO, storagePriceKey, int64(value))
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	cache.storagePrice = value
	return stackitem.Null{}
}

func (p *Policy) getAttributeFeeV0(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return p.getAttributeFeeGeneric(ic, args, false)
}

func (p *Policy) getAttributeFeeV1(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return p.getAttributeFeeGeneric(ic, args, true)
}

func (p *Policy) getAttributeFeeGeneric(ic *interop.Context, args []stackitem.Item, allowNotaryAssisted bool) stackitem.Item {
	t := transaction.AttrType(toUint8(args[0]))
	if !transaction.IsValidAttrType(ic.Chain.GetConfig().ReservedAttributes, t) ||
		(!allowNotaryAssisted && t == transaction.NotaryAssistedT) {
		panic(fmt.Errorf("invalid attribute type: %d", t))
	}
	return stackitem.NewBigInteger(big.NewInt(p.GetAttributeFeeInternal(ic.DAO, t)))
}

func (p *Policy) getBlockedAccounts(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	cache := ic.DAO.GetROCache(p.ID).(*PolicyCache)
	cloned := slices.Clone(cache.blockedAccounts)
	return stackitem.NewInterop(&iterator[util.Uint160]{keys: cloned})
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

func (p *Policy) setAttributeFeeV0(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return p.setAttributeFeeGeneric(ic, args, false)
}

func (p *Policy) setAttributeFeeV1(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return p.setAttributeFeeGeneric(ic, args, true)
}

func (p *Policy) setAttributeFeeGeneric(ic *interop.Context, args []stackitem.Item, allowNotaryAssisted bool) stackitem.Item {
	t := transaction.AttrType(toUint8(args[0]))
	value := toUint32(args[1])
	if !transaction.IsValidAttrType(ic.Chain.GetConfig().ReservedAttributes, t) ||
		(!allowNotaryAssisted && t == transaction.NotaryAssistedT) {
		panic(fmt.Errorf("invalid attribute type: %d", t))
	}
	if value > maxAttributeFee {
		panic(fmt.Errorf("attribute value is out of range: %d", value))
	}
	if !p.NEO.CheckCommittee(ic) {
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
	if !p.NEO.CheckCommittee(ic) {
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
	if !p.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}
	hash := toUint160(args[0])
	for i := range ic.Natives {
		if ic.Natives[i].Metadata().Hash == hash {
			panic("cannot block native contract")
		}
	}
	return stackitem.NewBool(p.BlockAccountInternal(ic, hash))
}

func (p *Policy) BlockAccountInternal(ic *interop.Context, hash util.Uint160) bool {
	i, blocked := p.isBlockedInternal(ic.DAO.GetROCache(p.ID).(*PolicyCache), hash)
	if blocked {
		return false
	}
	if ic.IsHardforkEnabled(config.HFFaun) {
		var _ = p.NEO.RevokeVotes(ic, hash) // ignore error, as in the reference.
	}
	key := append([]byte{blockedAccountPrefix}, hash.BytesBE()...)
	ic.DAO.PutStorageItem(p.ID, key, state.StorageItem{})
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
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
	if !p.NEO.CheckCommittee(ic) {
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

func (p *Policy) getMaxValidUntilBlockIncrement(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetMaxValidUntilBlockIncrementFromCache(ic.DAO))))
}

// GetMaxValidUntilBlockIncrementInternal returns current MaxValidUntilBlockIncrement.
// It respects Echidna enabling height.
func (p *Policy) GetMaxValidUntilBlockIncrementInternal(ic *interop.Context) uint32 {
	if ic.IsHardforkEnabled(config.HFEchidna) {
		return p.GetMaxValidUntilBlockIncrementFromCache(ic.DAO)
	}
	return ic.Chain.GetConfig().MaxValidUntilBlockIncrement
}

// GetMaxValidUntilBlockIncrementFromCache returns current MaxValidUntilBlockIncrement.
// It doesn't check neither Echidna enabling height nor cache initialization, so it's
// the caller's duty to ensure that Echidna is enabled before a call to this method.
func (p *Policy) GetMaxValidUntilBlockIncrementFromCache(d *dao.Simple) uint32 {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	return cache.maxVUBIncrement
}

func (p *Policy) setMaxValidUntilBlockIncrement(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	if value <= 0 || maxMaxVUBIncrement < value {
		panic(fmt.Errorf("MaxValidUntilBlockIncrement should be positive and not greater than %d, got %d", maxMaxVUBIncrement, value))
	}
	mtb := p.GetMaxTraceableBlocksInternal(ic.DAO)
	if value >= mtb {
		panic(fmt.Errorf("MaxValidUntilBlockIncrement should be less than MaxTraceableBlocks %d, got %d", mtb, value))
	}
	if !p.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}
	setIntWithKey(p.ID, ic.DAO, maxVUBIncrementKey, int64(value))
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	cache.maxVUBIncrement = value

	return stackitem.Null{}
}

func (p *Policy) getMillisecondsPerBlock(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetMillisecondsPerBlockInternal(ic.DAO))))
}

// GetMillisecondsPerBlockInternal returns current block generation time in milliseconds.
func (p *Policy) GetMillisecondsPerBlockInternal(d *dao.Simple) uint32 {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	return cache.msPerBlock
}

func (p *Policy) setMillisecondsPerBlock(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	if value <= 0 || maxMillisecondsPerBlock < value {
		panic(fmt.Errorf("MillisecondsPerBlock should be positive and not greater than %d, got %d", maxMillisecondsPerBlock, value))
	}
	if !p.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}
	setIntWithKey(p.ID, ic.DAO, msPerBlockKey, int64(value))
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	old := cache.msPerBlock
	cache.msPerBlock = value

	err := ic.AddNotification(p.Hash, "MillisecondsPerBlockChanged", stackitem.NewArray([]stackitem.Item{
		stackitem.NewBigInteger(big.NewInt(int64(old))),
		stackitem.NewBigInteger(big.NewInt(int64(value))),
	}))
	if err != nil {
		panic(err)
	}

	return stackitem.Null{}
}

func (p *Policy) getMaxTraceableBlocks(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(int64(p.GetMaxTraceableBlocksInternal(ic.DAO))))
}

// GetMaxTraceableBlocksInternal returns current MaxValidUntilBlockIncrement.
func (p *Policy) GetMaxTraceableBlocksInternal(d *dao.Simple) uint32 {
	cache := d.GetROCache(p.ID).(*PolicyCache)
	return cache.maxTraceableBlocks
}

func (p *Policy) setMaxTraceableBlocks(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toUint32(args[0])
	if value <= 0 || maxMaxTraceableBlocks < value {
		panic(fmt.Errorf("MaxTraceableBlocks should be positive and not greater than %d, got %d", maxMaxTraceableBlocks, value))
	}
	old := p.GetMaxTraceableBlocksInternal(ic.DAO)
	if value > old {
		panic(fmt.Errorf("MaxTraceableBlocks should not be greater than previous value %d, got %d", old, value))
	}
	maxVUBInc := p.GetMaxValidUntilBlockIncrementFromCache(ic.DAO)
	if value <= maxVUBInc {
		panic(fmt.Errorf("MaxTraceableBlocks should be larger than MaxValidUntilBlockIncrement %d, got %d", maxVUBInc, value))
	}
	if !p.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}

	setIntWithKey(p.ID, ic.DAO, MaxTraceableBlocksKey, int64(value))
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	cache.maxTraceableBlocks = value

	return stackitem.Null{}
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

// WhitelistedFee checks whether the specified contract method is whitelisted and
// returns a non-negative execution fee in picoGAS units if so. It always uses
// native cache, hence cache is expected to be initialized by this moment.
func (p *Policy) WhitelistedFee(dao *dao.Simple, hash util.Uint160, offset int) int64 {
	cache := dao.GetROCache(p.ID).(*PolicyCache)
	i, ok := slices.BinarySearchFunc(cache.whitelistedContracts, whitelistedContract{
		WhitelistFeeContract: state.WhitelistFeeContract{Hash: hash}, // hash+offset is enough for whitelistedContract.Compare.
		offset:               uint32(offset),
	}, whitelistedContract.Compare)
	if !ok {
		return -1
	}
	return cache.whitelistedContracts[i].Fee * vm.ExecFeeFactorMultiplier
}

// CleanWhitelist removes a contract with the specified hash from the list of
// whitelisted contracts.
func (p *Policy) CleanWhitelist(ic *interop.Context, h util.Uint160) error {
	var (
		cache = ic.DAO.GetRWCache(p.ID).(*PolicyCache)
		err   error
	)
	cache.whitelistedContracts = slices.DeleteFunc(cache.whitelistedContracts, func(c whitelistedContract) bool {
		if err != nil {
			return false
		}
		if !c.Hash.Equals(h) {
			return false
		}
		k := makeWhitelistedKey(c.Hash, c.offset)
		ic.DAO.DeleteStorageItem(p.ID, k)
		err = ic.AddNotification(p.Hash, "WhitelistFeeChanged", stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(c.Hash.BytesBE()),
			stackitem.NewByteArray([]byte(c.Method)),
			stackitem.NewBigInteger(big.NewInt(int64(c.ArgCnt))),
			stackitem.Null{},
		}))

		return err == nil
	})

	return err
}

func (p *Policy) removeWhitelistFeeContract(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	if !p.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}
	h := toUint160(args[0])
	method := toString(args[1])
	argCnt := int(toInt64(args[2]))

	cs, err := ic.GetContract(h)
	if err != nil {
		panic(fmt.Errorf("failed to get contract %s: %w", h.StringLE(), err))
	}
	md := cs.Manifest.ABI.GetMethod(method, argCnt)
	if md == nil {
		panic(fmt.Errorf("method not found: %s/%d", method, argCnt))
	}

	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	k := makeWhitelistedKey(h, uint32(md.Offset))
	i, ok := slices.BinarySearchFunc(cache.whitelistedContracts, whitelistedContract{
		WhitelistFeeContract: state.WhitelistFeeContract{Hash: h}, // hash+offset is enough for whitelistedContract.Compare.
		offset:               uint32(md.Offset),
	}, whitelistedContract.Compare)
	if !ok {
		panic(fmt.Errorf("whitelist for %s/%d not found", h.StringLE(), md.Offset))
	}
	ic.DAO.DeleteStorageItem(p.ID, k)
	cache.whitelistedContracts = slices.Delete(cache.whitelistedContracts, i, i+1)

	err = ic.AddNotification(p.Hash, "WhitelistFeeChanged", stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(h.BytesBE()),
		stackitem.NewByteArray([]byte(method)),
		stackitem.NewBigInteger(big.NewInt(int64(argCnt))),
		stackitem.Null{},
	}))
	if err != nil {
		panic(err)
	}

	return stackitem.Null{}
}

func (p *Policy) setWhitelistFeeContract(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	h := toUint160(args[0])
	method := toString(args[1])
	argCnt := int(toInt64(args[2]))
	fee := toInt64(args[3])
	if fee < 0 {
		panic(fmt.Errorf("fee should be positive, got %d", fee))
	}
	if !p.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}

	cs, err := ic.GetContract(h)
	if err != nil {
		panic(fmt.Errorf("contract %s not found: %w", h.StringLE(), err))
	}
	md := cs.Manifest.ABI.GetMethod(method, argCnt)
	if md == nil {
		panic(fmt.Errorf("contract %s: method not found: %s/%d", h.StringLE(), method, argCnt))
	}

	r := &state.WhitelistFeeContract{
		Hash:   h,
		Method: method,
		ArgCnt: argCnt,
		Fee:    fee,
	}
	err = ic.DAO.PutStorageConvertible(p.ID, makeWhitelistedKey(h, uint32(md.Offset)), r)
	if err != nil {
		panic(err)
	}
	cache := ic.DAO.GetRWCache(p.ID).(*PolicyCache)
	c := whitelistedContract{
		WhitelistFeeContract: *r,
		offset:               uint32(md.Offset),
	}
	i, ok := slices.BinarySearchFunc(cache.whitelistedContracts, c, whitelistedContract.Compare)
	if !ok {
		cache.whitelistedContracts = slices.Insert(cache.whitelistedContracts, i, c)
	}

	err = ic.AddNotification(p.Hash, "WhitelistFeeChanged", stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(h.BytesBE()),
		stackitem.NewByteArray([]byte(method)),
		stackitem.NewBigInteger(big.NewInt(int64(argCnt))),
		stackitem.NewBigInteger(big.NewInt(fee)),
	}))
	if err != nil {
		panic(err)
	}

	return stackitem.Null{}
}

func (p *Policy) getWhitelistFeeContracts(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	cache := ic.DAO.GetROCache(p.ID).(*PolicyCache)
	res := make([]*state.WhitelistFeeContract, len(cache.whitelistedContracts))
	for i, c := range cache.whitelistedContracts {
		res[i] = &c.WhitelistFeeContract
	}
	return stackitem.NewInterop(&iterator[*state.WhitelistFeeContract]{keys: res})
}

func makeWhitelistedKey(h util.Uint160, offset uint32) []byte {
	k := make([]byte, 1+util.Uint160Size+4)
	k[0] = whitelistedFeeContractPrefix
	copy(k[1:], h.BytesBE())
	binary.BigEndian.PutUint32(k[1+util.Uint160Size:], uint32(offset))
	return k
}

// iterator provides an iterator over a slice of T. T must be convertible to
// [stackitem.Item] via [stackitem.Make].
type iterator[T any] struct {
	keys []T
	next bool
}

// Next advances the iterator and returns true if Value can be called at the
// current position.
func (i *iterator[T]) Next() bool {
	if i.next {
		i.keys = i.keys[1:]
	}
	i.next = len(i.keys) > 0
	return i.next
}

// Value returns current iterators value.
func (i *iterator[T]) Value() stackitem.Item {
	if !i.next {
		panic("iterator index out of range")
	}
	return stackitem.Make(i.keys[0])
}
