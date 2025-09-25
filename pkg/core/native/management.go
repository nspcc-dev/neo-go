package native

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/bitfield"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Management is a contract-managing native contract.
type Management struct {
	interop.ContractMD
	NEO    INEO
	Policy IPolicy
}

type ManagementCache struct {
	contracts map[util.Uint160]*state.Contract
	// nep11 is a map of NEP11-compliant contracts which is updated with every PostPersist.
	nep11 map[util.Uint160]struct{}
	// nep17 is a map of NEP-17-compliant contracts which is updated with every PostPersist.
	nep17 map[util.Uint160]struct{}
}

const (
	// PrefixContract is a prefix used to store contract states inside Management native contract.
	PrefixContract     = 8
	prefixContractHash = 12

	defaultMinimumDeploymentFee     = 10_00000000
	contractDeployNotificationName  = "Deploy"
	contractUpdateNotificationName  = "Update"
	contractDestroyNotificationName = "Destroy"
)

var (
	errGasLimitExceeded = errors.New("gas limit exceeded")

	keyNextAvailableID      = []byte{15}
	keyMinimumDeploymentFee = []byte{20}
)

var (
	_ interop.Contract        = (*Management)(nil)
	_ dao.NativeContractCache = (*ManagementCache)(nil)
)

// Copy implements NativeContractCache interface.
func (c *ManagementCache) Copy() dao.NativeContractCache {
	cp := &ManagementCache{
		contracts: make(map[util.Uint160]*state.Contract),
		nep11:     make(map[util.Uint160]struct{}),
		nep17:     make(map[util.Uint160]struct{}),
	}
	// Copy the whole set of contracts is too expensive. We will create a separate map
	// holding the same set of pointers to contracts, and in case if some contract is
	// supposed to be changed, Management will create the copy in-place.
	for hash, ctr := range c.contracts {
		cp.contracts[hash] = ctr
	}
	for hash := range c.nep17 {
		cp.nep17[hash] = struct{}{}
	}
	for hash := range c.nep11 {
		cp.nep11[hash] = struct{}{}
	}
	return cp
}

// MakeContractKey creates a key from the account script hash.
func MakeContractKey(h util.Uint160) []byte {
	return makeUint160Key(PrefixContract, h)
}

// NewManagement creates a new Management native contract.
func NewManagement() *Management {
	var m = &Management{
		ContractMD: *interop.NewContractMD(nativenames.Management, nativeids.ContractManagement),
	}
	defer m.BuildHFSpecificMD(m.ActiveIn())

	desc := NewDescriptor("getContract", smartcontract.ArrayType,
		manifest.NewParameter("hash", smartcontract.Hash160Type))
	md := NewMethodAndPrice(m.getContract, 1<<15, callflag.ReadStates)
	m.AddMethod(md, desc)

	desc = NewDescriptor("deploy", smartcontract.ArrayType,
		manifest.NewParameter("nefFile", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(m.deploy, 0, callflag.All)
	m.AddMethod(md, desc)

	desc = NewDescriptor("deploy", smartcontract.ArrayType,
		manifest.NewParameter("nefFile", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType),
		manifest.NewParameter("data", smartcontract.AnyType))
	md = NewMethodAndPrice(m.deployWithData, 0, callflag.All)
	m.AddMethod(md, desc)

	desc = NewDescriptor("update", smartcontract.VoidType,
		manifest.NewParameter("nefFile", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(m.update, 0, callflag.All)
	m.AddMethod(md, desc)

	desc = NewDescriptor("update", smartcontract.VoidType,
		manifest.NewParameter("nefFile", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType),
		manifest.NewParameter("data", smartcontract.AnyType))
	md = NewMethodAndPrice(m.updateWithData, 0, callflag.All)
	m.AddMethod(md, desc)

	desc = NewDescriptor("destroy", smartcontract.VoidType)
	md = NewMethodAndPrice(m.destroy, 1<<15, callflag.States|callflag.AllowNotify)
	m.AddMethod(md, desc)

	desc = NewDescriptor("getMinimumDeploymentFee", smartcontract.IntegerType)
	md = NewMethodAndPrice(m.getMinimumDeploymentFee, 1<<15, callflag.ReadStates)
	m.AddMethod(md, desc)

	desc = NewDescriptor("setMinimumDeploymentFee", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = NewMethodAndPrice(m.setMinimumDeploymentFee, 1<<15, callflag.States)
	m.AddMethod(md, desc)

	desc = NewDescriptor("hasMethod", smartcontract.BoolType,
		manifest.NewParameter("hash", smartcontract.Hash160Type),
		manifest.NewParameter("method", smartcontract.StringType),
		manifest.NewParameter("pcount", smartcontract.IntegerType))
	md = NewMethodAndPrice(m.hasMethod, 1<<15, callflag.ReadStates)
	m.AddMethod(md, desc)

	desc = NewDescriptor("getContractById", smartcontract.ArrayType,
		manifest.NewParameter("id", smartcontract.IntegerType))
	md = NewMethodAndPrice(m.getContractByID, 1<<15, callflag.ReadStates)
	m.AddMethod(md, desc)

	desc = NewDescriptor("getContractHashes", smartcontract.InteropInterfaceType)
	md = NewMethodAndPrice(m.getContractHashes, 1<<15, callflag.ReadStates)
	m.AddMethod(md, desc)

	hashParam := manifest.NewParameter("Hash", smartcontract.Hash160Type)
	eDesc := NewEventDescriptor(contractDeployNotificationName, hashParam)
	eMD := NewEvent(eDesc)
	m.AddEvent(eMD)

	eDesc = NewEventDescriptor(contractUpdateNotificationName, hashParam)
	eMD = NewEvent(eDesc)
	m.AddEvent(eMD)

	eDesc = NewEventDescriptor(contractDestroyNotificationName, hashParam)
	eMD = NewEvent(eDesc)
	m.AddEvent(eMD)

	desc = NewDescriptor("isContract", smartcontract.BoolType,
		manifest.NewParameter("hash", smartcontract.Hash160Type))
	md = NewMethodAndPrice(m.isContract, 1<<14, callflag.ReadStates, config.HFEchidna)
	m.AddMethod(md, desc)

	return m
}

func toHash160(si stackitem.Item) util.Uint160 {
	hashBytes, err := si.TryBytes()
	if err != nil {
		panic(err)
	}
	hash, err := util.Uint160DecodeBytesBE(hashBytes)
	if err != nil {
		panic(err)
	}
	return hash
}

// getContract is an implementation of public getContract method, it's run under
// VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) getContract(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	hash := toHash160(args[0])
	ctr, err := GetContract(ic.DAO, m.ID, hash)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return stackitem.Null{}
		}
		panic(err)
	}
	return contractToStack(ctr)
}

// getContractByID is an implementation of public getContractById method, it's run under
// VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) getContractByID(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	idBig, err := args[0].TryInteger()
	if err != nil {
		panic(err)
	}
	id := idBig.Int64()
	if !idBig.IsInt64() || id < math.MinInt32 || id > math.MaxInt32 {
		panic("id is not a correct int32")
	}
	ctr, err := GetContractByID(ic.DAO, m.ID, int32(id))
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return stackitem.Null{}
		}
		panic(err)
	}
	return contractToStack(ctr)
}

// GetContract returns a contract with the given hash from the given DAO.
func GetContract(d *dao.Simple, managementID int32, hash util.Uint160) (*state.Contract, error) {
	cache := d.GetROCache(managementID).(*ManagementCache)
	return getContract(cache, hash)
}

// getContract returns a contract with the given hash from provided RO or RW cache.
func getContract(cache *ManagementCache, hash util.Uint160) (*state.Contract, error) {
	cs, ok := cache.contracts[hash]
	if !ok {
		return nil, storage.ErrKeyNotFound
	}
	return cs, nil
}

// GetContractByID returns a contract with the given ID from the given DAO.
func GetContractByID(d *dao.Simple, managementID int32, id int32) (*state.Contract, error) {
	hash, err := GetContractScriptHash(d, managementID, id)
	if err != nil {
		return nil, err
	}
	return GetContract(d, managementID, hash)
}

// GetContractScriptHash returns a contract hash associated with the given ID from the given DAO.
func GetContractScriptHash(d *dao.Simple, managementID int32, id int32) (util.Uint160, error) {
	key := make([]byte, 5)
	key = putHashKey(key, id)
	si := d.GetStorageItem(managementID, key)
	if si == nil {
		return util.Uint160{}, storage.ErrKeyNotFound
	}
	return util.Uint160DecodeBytesBE(si)
}

func getLimitedSlice(arg stackitem.Item, maxLen int) ([]byte, error) {
	_, isNull := arg.(stackitem.Null)
	if isNull {
		return nil, nil
	}
	b, err := arg.TryBytes()
	if err != nil {
		return nil, err
	}
	l := len(b)
	if l == 0 {
		return nil, errors.New("empty")
	} else if l > maxLen {
		return nil, fmt.Errorf("len is %d (max %d)", l, maxLen)
	}

	return b, nil
}

func (m *Management) getContractHashes(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	ctx, cancel := context.WithCancel(context.Background())
	prefix := []byte{prefixContractHash}
	seekres := ic.DAO.SeekAsync(ctx, ic.Chain.NativeManagementID(), storage.SeekRange{Prefix: prefix})
	filteredRes := make(chan storage.KeyValue)
	go func() {
	loop:
		for kv := range seekres {
			if len(kv.Key) == 4 && binary.BigEndian.Uint32(kv.Key) < math.MaxInt32 {
				select {
				case <-ctx.Done():
					break loop
				case filteredRes <- kv:
				}
			}
		}
		close(filteredRes)
	}()
	opts := istorage.FindRemovePrefix
	item := istorage.NewIterator(filteredRes, prefix, int64(opts))
	ic.RegisterCancelFunc(func() {
		cancel()
		for range filteredRes { //nolint:revive //empty-block
		}
	})
	return stackitem.NewInterop(item)
}

// getNefAndManifestFromItems converts input arguments into NEF and manifest
// adding an appropriate deployment GAS price and sanitizing inputs.
func (m *Management) getNefAndManifestFromItems(ic *interop.Context, args []stackitem.Item, isDeploy bool) (*nef.File, *manifest.Manifest, error) {
	nefBytes, err := getLimitedSlice(args[0], math.MaxInt32) // Upper limits are checked during NEF deserialization.
	if err != nil {
		return nil, nil, fmt.Errorf("invalid NEF file: %w", err)
	}
	manifestBytes, err := getLimitedSlice(args[1], manifest.MaxManifestSize)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid manifest: %w", err)
	}

	gas := ic.BaseStorageFee() * int64(len(nefBytes)+len(manifestBytes))
	if isDeploy {
		fee := m.minimumDeploymentFee(ic.DAO)
		if fee > gas {
			gas = fee
		}
	}
	if !ic.VM.AddGas(gas) {
		return nil, nil, errGasLimitExceeded
	}
	var resManifest *manifest.Manifest
	var resNef *nef.File
	if nefBytes != nil {
		nf, err := nef.FileFromBytes(nefBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid NEF file: %w", err)
		}
		resNef = &nf
	}
	if manifestBytes != nil {
		if !utf8.Valid(manifestBytes) {
			return nil, nil, errors.New("manifest is not UTF-8 compliant")
		}
		resManifest = new(manifest.Manifest)
		err := json.Unmarshal(manifestBytes, resManifest)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid manifest: %w", err)
		}
	}
	return resNef, resManifest, nil
}

// deploy is an implementation of public 2-argument deploy method.
func (m *Management) deploy(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return m.deployWithData(ic, append(args, stackitem.Null{}))
}

// deployWithData is an implementation of public 3-argument deploy method.
// It's run under VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) deployWithData(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	neff, manif, err := m.getNefAndManifestFromItems(ic, args, true)
	if err != nil {
		panic(err)
	}
	if neff == nil {
		panic(errors.New("no valid NEF provided"))
	}
	if manif == nil {
		panic(errors.New("no valid manifest provided"))
	}
	if ic.Tx == nil {
		panic(errors.New("no transaction provided"))
	}
	newcontract, err := m.Deploy(ic, ic.Tx.Sender(), neff, manif)
	if err != nil {
		panic(err)
	}
	m.callDeploy(ic, newcontract, args[2], false)
	err = m.emitNotification(ic, contractDeployNotificationName, newcontract.Hash)
	if err != nil {
		panic(err)
	}
	return contractToStack(newcontract)
}

func markUpdated(d *dao.Simple, managementID int32, hash util.Uint160, cs *state.Contract) {
	cache := d.GetRWCache(managementID).(*ManagementCache)
	delete(cache.nep11, hash)
	delete(cache.nep17, hash)
	if cs == nil {
		delete(cache.contracts, hash)
		return
	}
	updateContractCache(cache, cs)
}

// Deploy creates a contract's hash/ID and saves a new contract into the given DAO.
// It doesn't run _deploy method and doesn't emit notification.
func (m *Management) Deploy(ic *interop.Context, sender util.Uint160, neff *nef.File, manif *manifest.Manifest) (*state.Contract, error) {
	h := state.CreateContractHash(sender, neff.Checksum, manif.Name)
	if m.Policy.IsBlocked(ic.DAO, h) {
		return nil, fmt.Errorf("the contract %s has been blocked", h.StringLE())
	}
	_, err := GetContract(ic.DAO, m.ID, h)
	if err == nil {
		return nil, errors.New("contract already exists")
	}
	id, err := m.getNextContractID(ic.DAO)
	if err != nil {
		return nil, err
	}
	err = manif.IsValid(h, false) // do not check manifest size, the whole state.Contract will be checked later.
	if err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	err = checkScriptAndMethods(ic, neff.Script, manif.ABI.Methods)
	if err != nil {
		return nil, err
	}
	newcontract := &state.Contract{
		ContractBase: state.ContractBase{
			ID:       id,
			Hash:     h,
			NEF:      *neff,
			Manifest: *manif,
		},
	}
	err = PutContractState(ic.DAO, m.ID, newcontract)
	if err != nil {
		return nil, err
	}
	return newcontract, nil
}

func (m *Management) update(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return m.updateWithData(ic, append(args, stackitem.Null{}))
}

// update is an implementation of public update method, it's run under
// VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) updateWithData(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	neff, manif, err := m.getNefAndManifestFromItems(ic, args, false)
	if err != nil {
		panic(err)
	}
	if neff == nil && manif == nil {
		panic(errors.New("both NEF and manifest are nil"))
	}
	contract, err := m.Update(ic, ic.VM.GetCallingScriptHash(), neff, manif)
	if err != nil {
		panic(err)
	}
	m.callDeploy(ic, contract, args[2], true)
	err = m.emitNotification(ic, contractUpdateNotificationName, contract.Hash)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// Update updates contract's script and/or manifest in the given DAO.
// It doesn't run _deploy method and doesn't emit notification.
func (m *Management) Update(ic *interop.Context, hash util.Uint160, neff *nef.File, manif *manifest.Manifest) (*state.Contract, error) {
	var contract state.Contract

	oldcontract, err := GetContract(ic.DAO, m.ID, hash)
	if err != nil {
		return nil, errors.New("contract doesn't exist")
	}
	if oldcontract.UpdateCounter == math.MaxUint16 {
		return nil, errors.New("the contract reached the maximum number of updates")
	}

	contract = *oldcontract // Make a copy, don't ruin (potentially) cached contract.
	// if NEF was provided, update the contract script
	if neff != nil {
		contract.NEF = *neff
	}
	// if manifest was provided, update the contract manifest
	if manif != nil {
		if manif.Name != contract.Manifest.Name {
			return nil, errors.New("contract name can't be changed")
		}
		err = manif.IsValid(contract.Hash, false) // do not check manifest size, the whole state.Contract will be checked later.
		if err != nil {
			return nil, fmt.Errorf("invalid manifest: %w", err)
		}
		contract.Manifest = *manif
	}
	err = checkScriptAndMethods(ic, contract.NEF.Script, contract.Manifest.ABI.Methods)
	if err != nil {
		return nil, err
	}
	contract.UpdateCounter++
	err = PutContractState(ic.DAO, m.ID, &contract)
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

// destroy is an implementation of destroy update method, it's run under
// VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) destroy(ic *interop.Context, sis []stackitem.Item) stackitem.Item {
	hash := ic.VM.GetCallingScriptHash()
	err := m.Destroy(ic.DAO, hash)
	if err != nil {
		panic(err)
	}
	err = m.emitNotification(ic, contractDestroyNotificationName, hash)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// Destroy drops the given contract from DAO along with its storage. It doesn't emit notification.
func (m *Management) Destroy(d *dao.Simple, hash util.Uint160) error {
	contract, err := GetContract(d, m.ID, hash)
	if err != nil {
		return err
	}
	key := MakeContractKey(hash)
	d.DeleteStorageItem(m.ID, key)
	key = putHashKey(key, contract.ID)
	d.DeleteStorageItem(m.ID, key)

	d.Seek(contract.ID, storage.SeekRange{}, func(k, _ []byte) bool {
		d.DeleteStorageItem(contract.ID, k)
		return true
	})
	m.Policy.BlockAccountInternal(d, hash)
	markUpdated(d, m.ID, hash, nil)
	return nil
}

func (m *Management) getMinimumDeploymentFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(m.minimumDeploymentFee(ic.DAO)))
}

// minimumDeploymentFee returns the minimum required fee for contract deploy.
func (m *Management) minimumDeploymentFee(dao *dao.Simple) int64 {
	return getIntWithKey(m.ID, dao, keyMinimumDeploymentFee)
}

func (m *Management) setMinimumDeploymentFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toBigInt(args[0])
	if value.Sign() < 0 {
		panic("MinimumDeploymentFee cannot be negative")
	}
	if !m.NEO.CheckCommittee(ic) {
		panic("invalid committee signature")
	}
	ic.DAO.PutStorageItem(m.ID, keyMinimumDeploymentFee, bigint.ToBytes(value))
	return stackitem.Null{}
}

func (m *Management) callDeploy(ic *interop.Context, cs *state.Contract, data stackitem.Item, isUpdate bool) {
	md := cs.Manifest.ABI.GetMethod(manifest.MethodDeploy, 2)
	if md != nil {
		err := contract.CallFromNative(ic, m.Hash, cs, manifest.MethodDeploy,
			[]stackitem.Item{data, stackitem.NewBool(isUpdate)}, false)
		if err != nil {
			panic(err)
		}
	}
}

func contractToStack(cs *state.Contract) stackitem.Item {
	si, err := cs.ToStackItem()
	if err != nil {
		panic(fmt.Errorf("contract to stack item: %w", err))
	}
	return si
}

func (m *Management) hasMethod(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	cHash := toHash160(args[0])
	method, err := stackitem.ToString(args[1])
	if err != nil {
		panic(err)
	}
	pcount := int(toInt64((args[2])))
	cs, err := GetContract(ic.DAO, m.ID, cHash)
	if err != nil {
		return stackitem.NewBool(false)
	}
	return stackitem.NewBool(cs.Manifest.ABI.GetMethod(method, pcount) != nil)
}

func (m *Management) isContract(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	h := toHash160(args[0])
	_, err := GetContract(ic.DAO, m.ID, h)
	return stackitem.NewBool(err == nil)
}

// Metadata implements the Contract interface.
func (m *Management) Metadata() *interop.ContractMD {
	return &m.ContractMD
}

// updateContractCache saves the contract in the common and NEP-related caches. It's
// an internal method that must be called with m.mtx lock taken.
func updateContractCache(cache *ManagementCache, cs *state.Contract) {
	cache.contracts[cs.Hash] = cs
	if cs.Manifest.IsStandardSupported(manifest.NEP11StandardName) {
		cache.nep11[cs.Hash] = struct{}{}
	}
	if cs.Manifest.IsStandardSupported(manifest.NEP17StandardName) {
		cache.nep17[cs.Hash] = struct{}{}
	}
}

// OnPersist implements the Contract interface.
func (m *Management) OnPersist(ic *interop.Context) error {
	var cache *ManagementCache
	for _, native := range ic.Natives {
		var (
			activeIn         = native.ActiveIn()
			isDeploy         bool
			isUpdate         bool
			latestHF         config.Hardfork
			currentActiveHFs []config.Hardfork
		)
		activeHFs := native.Metadata().ActiveHFs
		isDeploy = activeIn == nil && ic.Block.Index == 0 ||
			activeIn != nil && ic.IsHardforkActivation(*activeIn)
		if !isDeploy {
			for _, hf := range config.Hardforks {
				if _, ok := activeHFs[hf]; ok && ic.IsHardforkActivation(hf) {
					isUpdate = true
					// Break immediately since native Initialize should be called starting from the first hardfork in a raw
					// (if there are multiple hardforks with the same enabling height).
					break
				}
			}
		}
		// Search for the latest active hardfork to properly construct manifest and
		// initialize natives for the range of active hardforks.
		for _, hf := range config.Hardforks {
			if _, ok := activeHFs[hf]; ok && ic.IsHardforkActivation(hf) {
				latestHF = hf
				currentActiveHFs = append(currentActiveHFs, hf)
			}
		}
		// activeIn is not included into the activeHFs list.
		if activeIn != nil && activeIn.Cmp(latestHF) > 0 {
			latestHF = *activeIn
		}
		if !isDeploy && !isUpdate {
			continue
		}
		md := native.Metadata()
		hfSpecificMD := md.HFSpecificContractMD(&latestHF)
		base := hfSpecificMD.ContractBase
		var cs *state.Contract
		switch {
		case isDeploy:
			cs = &state.Contract{
				ContractBase: base,
			}
		case isUpdate:
			if cache == nil {
				cache = ic.DAO.GetRWCache(m.ID).(*ManagementCache)
			}
			oldcontract, err := getContract(cache, md.Hash)
			if err != nil {
				return fmt.Errorf("failed to retrieve native %s from cache: %w", md.Name, err)
			}

			contract := *oldcontract // Make a copy, don't ruin cached contract and cache.
			contract.NEF = base.NEF
			contract.Manifest = base.Manifest
			contract.UpdateCounter++
			cs = &contract
		}
		err := putContractState(ic.DAO, m.ID, cs, false) // Perform cache update manually.
		if err != nil {
			return fmt.Errorf("failed to put contract state: %w", err)
		}

		// Deploy hardfork (contract's ActiveIn) is not a part of contract's active hardforks and
		// allowed to be nil, this, a special initialization call for it.
		if isDeploy {
			if err := native.Initialize(ic, activeIn, hfSpecificMD); err != nil {
				return fmt.Errorf("initializing %s native contract at HF %v: %w", md.Name, activeIn, err)
			}
		}
		// The rest of activating hardforks also require initialization.
		for _, hf := range currentActiveHFs {
			if err := native.Initialize(ic, &hf, hfSpecificMD); err != nil {
				return fmt.Errorf("initializing %s native contract at HF %s: %w", md.Name, hf, err)
			}
		}

		if cache == nil {
			cache = ic.DAO.GetRWCache(m.ID).(*ManagementCache)
		}
		updateContractCache(cache, cs)

		ntfName := contractDeployNotificationName
		if isUpdate {
			ntfName = contractUpdateNotificationName
		}
		err = m.emitNotification(ic, ntfName, cs.Hash)
		if err != nil {
			return err
		}
	}

	return nil
}

// InitializeCache initializes contract cache with the proper values from storage.
// Cache initialization should be done apart from Initialize because Initialize is
// called only when deploying native contracts.
func (m *Management) InitializeCache(_ interop.IsHardforkEnabled, blockHeight uint32, d *dao.Simple) error {
	cache := &ManagementCache{
		contracts: make(map[util.Uint160]*state.Contract),
		nep11:     make(map[util.Uint160]struct{}),
		nep17:     make(map[util.Uint160]struct{}),
	}

	var initErr error
	d.Seek(m.ID, storage.SeekRange{Prefix: []byte{PrefixContract}}, func(_, v []byte) bool {
		var cs = new(state.Contract)
		initErr = stackitem.DeserializeConvertible(v, cs)
		if initErr != nil {
			return false
		}
		updateContractCache(cache, cs)
		return true
	})
	if initErr != nil {
		return initErr
	}
	d.SetCache(m.ID, cache)
	return nil
}

// PostPersist implements the Contract interface.
func (m *Management) PostPersist(ic *interop.Context) error {
	return nil
}

// GetNEP11Contracts returns hashes of all deployed contracts that support NEP-11 standard. The list
// is updated every PostPersist, so until PostPersist is called, the result for the previous block
// is returned.
func (m *Management) GetNEP11Contracts(d *dao.Simple) []util.Uint160 {
	cache := d.GetROCache(m.ID).(*ManagementCache)
	result := make([]util.Uint160, 0, len(cache.nep11))
	for h := range cache.nep11 {
		result = append(result, h)
	}
	return result
}

// GetNEP17Contracts returns hashes of all deployed contracts that support NEP-17 standard. The list
// is updated every PostPersist, so until PostPersist is called, the result for the previous block
// is returned.
func (m *Management) GetNEP17Contracts(d *dao.Simple) []util.Uint160 {
	cache := d.GetROCache(m.ID).(*ManagementCache)
	result := make([]util.Uint160, 0, len(cache.nep17))
	for h := range cache.nep17 {
		result = append(result, h)
	}
	return result
}

// Initialize implements the Contract interface.
func (m *Management) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	if hf != m.ActiveIn() {
		return nil
	}

	setIntWithKey(m.ID, ic.DAO, keyMinimumDeploymentFee, defaultMinimumDeploymentFee)
	setIntWithKey(m.ID, ic.DAO, keyNextAvailableID, 1)

	cache := &ManagementCache{
		contracts: make(map[util.Uint160]*state.Contract),
		nep11:     make(map[util.Uint160]struct{}),
		nep17:     make(map[util.Uint160]struct{}),
	}
	ic.DAO.SetCache(m.ID, cache)
	return nil
}

// ActiveIn implements the Contract interface.
func (m *Management) ActiveIn() *config.Hardfork {
	return nil
}

// PutContractState saves given contract state into given DAO.
func PutContractState(d *dao.Simple, managementID int32, cs *state.Contract) error {
	return putContractState(d, managementID, cs, true)
}

// putContractState is an internal PutContractState representation.
func putContractState(d *dao.Simple, managementID int32, cs *state.Contract, updateCache bool) error {
	key := MakeContractKey(cs.Hash)
	if err := putConvertibleToDAO(managementID, d, key, cs); err != nil {
		return err
	}
	if updateCache {
		markUpdated(d, managementID, cs.Hash, cs)
	}
	if cs.UpdateCounter != 0 { // Update.
		return nil
	}
	key = putHashKey(key, cs.ID)
	d.PutStorageItem(managementID, key, cs.Hash.BytesBE())
	return nil
}

func putHashKey(buf []byte, id int32) []byte {
	buf[0] = prefixContractHash
	binary.BigEndian.PutUint32(buf[1:], uint32(id))
	return buf[:5]
}

func (m *Management) getNextContractID(d *dao.Simple) (int32, error) {
	si := d.GetStorageItem(m.ID, keyNextAvailableID)
	if si == nil {
		return 0, errors.New("nextAvailableID is not initialized")
	}
	id := bigint.FromBytes(si)
	ret := int32(id.Int64())
	id.Add(id, intOne)
	d.PutBigInt(m.ID, keyNextAvailableID, id)
	return ret, nil
}

func (m *Management) emitNotification(ic *interop.Context, name string, hash util.Uint160) error {
	return ic.AddNotification(m.Hash, name, stackitem.NewArray([]stackitem.Item{addrToStackItem(&hash)}))
}

func checkScriptAndMethods(ic *interop.Context, script []byte, methods []manifest.Method) error {
	l := len(script)
	offsets := bitfield.New(l)
	for i := range methods {
		if methods[i].Offset >= l {
			return fmt.Errorf("method %s/%d: offset is out of the script range", methods[i].Name, len(methods[i].Parameters))
		}
		offsets.Set(methods[i].Offset)
	}
	if !ic.IsHardforkEnabled(config.HFBasilisk) {
		return nil
	}
	err := vm.IsScriptCorrect(script, offsets)
	if err != nil {
		return fmt.Errorf("invalid contract script: %w", err)
	}

	return nil
}
