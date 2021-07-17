package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
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

// Management is contract-managing native contract.
type Management struct {
	interop.ContractMD
	NEO *NEO

	mtx       sync.RWMutex
	contracts map[util.Uint160]*state.Contract
}

const (
	managementContractID = -1

	prefixContract = 8

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

// makeContractKey creates a key from account script hash.
func makeContractKey(h util.Uint160) []byte {
	return makeUint160Key(prefixContract, h)
}

// newManagement creates new Management native contract.
func newManagement() *Management {
	var m = &Management{
		ContractMD: *interop.NewContractMD(nativenames.Management, managementContractID),
		contracts:  make(map[util.Uint160]*state.Contract),
	}
	defer m.UpdateHash()

	desc := newDescriptor("getContract", smartcontract.ArrayType,
		manifest.NewParameter("hash", smartcontract.Hash160Type))
	md := newMethodAndPrice(m.getContract, 1<<15, callflag.ReadStates)
	m.AddMethod(md, desc)

	desc = newDescriptor("deploy", smartcontract.ArrayType,
		manifest.NewParameter("nefFile", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType))
	md = newMethodAndPrice(m.deploy, 0, callflag.States|callflag.AllowNotify)
	m.AddMethod(md, desc)

	desc = newDescriptor("deploy", smartcontract.ArrayType,
		manifest.NewParameter("nefFile", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType),
		manifest.NewParameter("data", smartcontract.AnyType))
	md = newMethodAndPrice(m.deployWithData, 0, callflag.States|callflag.AllowNotify)
	m.AddMethod(md, desc)

	desc = newDescriptor("update", smartcontract.VoidType,
		manifest.NewParameter("nefFile", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType))
	md = newMethodAndPrice(m.update, 0, callflag.States|callflag.AllowNotify)
	m.AddMethod(md, desc)

	desc = newDescriptor("update", smartcontract.VoidType,
		manifest.NewParameter("nefFile", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType),
		manifest.NewParameter("data", smartcontract.AnyType))
	md = newMethodAndPrice(m.updateWithData, 0, callflag.States|callflag.AllowNotify)
	m.AddMethod(md, desc)

	desc = newDescriptor("destroy", smartcontract.VoidType)
	md = newMethodAndPrice(m.destroy, 1<<15, callflag.States|callflag.AllowNotify)
	m.AddMethod(md, desc)

	desc = newDescriptor("getMinimumDeploymentFee", smartcontract.IntegerType)
	md = newMethodAndPrice(m.getMinimumDeploymentFee, 1<<15, callflag.ReadStates)
	m.AddMethod(md, desc)

	desc = newDescriptor("setMinimumDeploymentFee", smartcontract.VoidType,
		manifest.NewParameter("value", smartcontract.IntegerType))
	md = newMethodAndPrice(m.setMinimumDeploymentFee, 1<<15, callflag.States)
	m.AddMethod(md, desc)

	hashParam := manifest.NewParameter("Hash", smartcontract.Hash160Type)
	m.AddEvent(contractDeployNotificationName, hashParam)
	m.AddEvent(contractUpdateNotificationName, hashParam)
	m.AddEvent(contractDestroyNotificationName, hashParam)
	return m
}

// getContract is an implementation of public getContract method, it's run under
// VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) getContract(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	hashBytes, err := args[0].TryBytes()
	if err != nil {
		panic(err)
	}
	hash, err := util.Uint160DecodeBytesBE(hashBytes)
	if err != nil {
		panic(err)
	}
	ctr, err := m.GetContract(ic.DAO, hash)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			return stackitem.Null{}
		}
		panic(err)
	}
	return contractToStack(ctr)
}

// GetContract returns contract with given hash from given DAO.
func (m *Management) GetContract(d dao.DAO, hash util.Uint160) (*state.Contract, error) {
	m.mtx.RLock()
	cs, ok := m.contracts[hash]
	m.mtx.RUnlock()
	if !ok {
		return nil, storage.ErrKeyNotFound
	} else if cs != nil {
		return cs, nil
	}
	return m.getContractFromDAO(d, hash)
}

func (m *Management) getContractFromDAO(d dao.DAO, hash util.Uint160) (*state.Contract, error) {
	contract := new(state.Contract)
	key := makeContractKey(hash)
	err := getConvertibleFromDAO(m.ID, d, key, contract)
	if err != nil {
		return nil, err
	}
	return contract, nil
}

func getLimitedSlice(arg stackitem.Item, max int) ([]byte, error) {
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
	} else if l > max {
		return nil, fmt.Errorf("len is %d (max %d)", l, max)
	}

	return b, nil
}

// getNefAndManifestFromItems converts input arguments into NEF and manifest
// adding appropriate deployment GAS price and sanitizing inputs.
func (m *Management) getNefAndManifestFromItems(ic *interop.Context, args []stackitem.Item, isDeploy bool) (*nef.File, *manifest.Manifest, error) {
	nefBytes, err := getLimitedSlice(args[0], math.MaxInt32) // Upper limits are checked during NEF deserialization.
	if err != nil {
		return nil, nil, fmt.Errorf("invalid NEF file: %w", err)
	}
	manifestBytes, err := getLimitedSlice(args[1], manifest.MaxManifestSize)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid manifest: %w", err)
	}

	gas := ic.Chain.GetPolicer().GetStoragePrice() * int64(len(nefBytes)+len(manifestBytes))
	if isDeploy {
		fee := m.GetMinimumDeploymentFee(ic.DAO)
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
	newcontract, err := m.Deploy(ic.DAO, ic.Tx.Sender(), neff, manif)
	if err != nil {
		panic(err)
	}
	m.callDeploy(ic, newcontract, args[2], false)
	m.emitNotification(ic, contractDeployNotificationName, newcontract.Hash)
	return contractToStack(newcontract)
}

func (m *Management) markUpdated(h util.Uint160) {
	m.mtx.Lock()
	// Just set it to nil, to refresh cache in `PostPersist`.
	m.contracts[h] = nil
	m.mtx.Unlock()
}

// Deploy creates contract's hash/ID and saves new contract into the given DAO.
// It doesn't run _deploy method and doesn't emit notification.
func (m *Management) Deploy(d dao.DAO, sender util.Uint160, neff *nef.File, manif *manifest.Manifest) (*state.Contract, error) {
	h := state.CreateContractHash(sender, neff.Checksum, manif.Name)
	key := makeContractKey(h)
	si := d.GetStorageItem(m.ID, key)
	if si != nil {
		return nil, errors.New("contract already exists")
	}
	id, err := m.getNextContractID(d)
	if err != nil {
		return nil, err
	}
	err = manif.IsValid(h)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	err = checkScriptAndMethods(neff.Script, manif.ABI.Methods)
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
	err = m.PutContractState(d, newcontract)
	if err != nil {
		return nil, err
	}
	m.markUpdated(newcontract.Hash)
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
	contract, err := m.Update(ic.DAO, ic.VM.GetCallingScriptHash(), neff, manif)
	if err != nil {
		panic(err)
	}
	m.callDeploy(ic, contract, args[2], true)
	m.emitNotification(ic, contractUpdateNotificationName, contract.Hash)
	return stackitem.Null{}
}

// Update updates contract's script and/or manifest in the given DAO.
// It doesn't run _deploy method and doesn't emit notification.
func (m *Management) Update(d dao.DAO, hash util.Uint160, neff *nef.File, manif *manifest.Manifest) (*state.Contract, error) {
	var contract state.Contract

	oldcontract, err := m.GetContract(d, hash)
	if err != nil {
		return nil, errors.New("contract doesn't exist")
	}

	contract = *oldcontract // Make a copy, don't ruin (potentially) cached contract.
	// if NEF was provided, update the contract script
	if neff != nil {
		m.markUpdated(hash)
		contract.NEF = *neff
	}
	// if manifest was provided, update the contract manifest
	if manif != nil {
		if manif.Name != contract.Manifest.Name {
			return nil, errors.New("contract name can't be changed")
		}
		err = manif.IsValid(contract.Hash)
		if err != nil {
			return nil, fmt.Errorf("invalid manifest: %w", err)
		}
		m.markUpdated(hash)
		contract.Manifest = *manif
	}
	err = checkScriptAndMethods(contract.NEF.Script, contract.Manifest.ABI.Methods)
	if err != nil {
		return nil, err
	}
	contract.UpdateCounter++
	err = m.PutContractState(d, &contract)
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
	m.emitNotification(ic, contractDestroyNotificationName, hash)
	return stackitem.Null{}
}

// Destroy drops given contract from DAO along with its storage. It doesn't emit notification.
func (m *Management) Destroy(d dao.DAO, hash util.Uint160) error {
	contract, err := m.GetContract(d, hash)
	if err != nil {
		return err
	}
	key := makeContractKey(hash)
	err = d.DeleteStorageItem(m.ID, key)
	if err != nil {
		return err
	}
	err = d.DeleteContractID(contract.ID)
	if err != nil {
		return err
	}
	siMap, err := d.GetStorageItems(contract.ID)
	if err != nil {
		return err
	}
	for k := range siMap {
		err := d.DeleteStorageItem(contract.ID, []byte(k))
		if err != nil {
			return err
		}
	}
	m.markUpdated(hash)
	return nil
}

func (m *Management) getMinimumDeploymentFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(m.GetMinimumDeploymentFee(ic.DAO)))
}

// GetMinimumDeploymentFee returns the minimum required fee for contract deploy.
func (m *Management) GetMinimumDeploymentFee(dao dao.DAO) int64 {
	return getIntWithKey(m.ID, dao, keyMinimumDeploymentFee)
}

func (m *Management) setMinimumDeploymentFee(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	value := toBigInt(args[0])
	if value.Sign() < 0 {
		panic("MinimumDeploymentFee cannot be negative")
	}
	if !m.NEO.checkCommittee(ic) {
		panic("invalid committee signature")
	}
	err := ic.DAO.PutStorageItem(m.ID, keyMinimumDeploymentFee, bigint.ToBytes(value))
	if err != nil {
		panic(err)
	}
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

// Metadata implements Contract interface.
func (m *Management) Metadata() *interop.ContractMD {
	return &m.ContractMD
}

// OnPersist implements Contract interface.
func (m *Management) OnPersist(ic *interop.Context) error {
	for _, native := range ic.Natives {
		md := native.Metadata()
		history := md.UpdateHistory
		if len(history) == 0 || history[0] != ic.Block.Index {
			continue
		}

		cs := &state.Contract{
			ContractBase: md.ContractBase,
		}
		err := m.PutContractState(ic.DAO, cs)
		if err != nil {
			return err
		}
		if err := native.Initialize(ic); err != nil {
			return fmt.Errorf("initializing %s native contract: %w", md.Name, err)
		}
		m.mtx.Lock()
		m.contracts[md.Hash] = cs
		m.mtx.Unlock()
	}

	return nil
}

// InitializeCache initializes contract cache with the proper values from storage.
// Cache initialisation should be done apart from Initialize because Initialize is
// called only when deploying native contracts.
func (m *Management) InitializeCache(d dao.DAO) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	var initErr error
	d.Seek(m.ID, []byte{prefixContract}, func(_, v []byte) {
		var cs = new(state.Contract)
		initErr = stackitem.DeserializeConvertible(v, cs)
		if initErr != nil {
			return
		}
		m.contracts[cs.Hash] = cs
	})
	return initErr
}

// PostPersist implements Contract interface.
func (m *Management) PostPersist(ic *interop.Context) error {
	m.mtx.Lock()
	for h, cs := range m.contracts {
		if cs != nil {
			continue
		}
		newCs, err := m.getContractFromDAO(ic.DAO, h)
		if err != nil {
			// Contract was destroyed.
			delete(m.contracts, h)
			continue
		}
		m.contracts[h] = newCs
	}
	m.mtx.Unlock()
	return nil
}

// Initialize implements Contract interface.
func (m *Management) Initialize(ic *interop.Context) error {
	if err := setIntWithKey(m.ID, ic.DAO, keyMinimumDeploymentFee, defaultMinimumDeploymentFee); err != nil {
		return err
	}
	return setIntWithKey(m.ID, ic.DAO, keyNextAvailableID, 1)
}

// PutContractState saves given contract state into given DAO.
func (m *Management) PutContractState(d dao.DAO, cs *state.Contract) error {
	key := makeContractKey(cs.Hash)
	if err := putConvertibleToDAO(m.ID, d, key, cs); err != nil {
		return err
	}
	m.markUpdated(cs.Hash)
	if cs.UpdateCounter != 0 { // Update.
		return nil
	}
	return d.PutContractID(cs.ID, cs.Hash)
}

func (m *Management) getNextContractID(d dao.DAO) (int32, error) {
	si := d.GetStorageItem(m.ID, keyNextAvailableID)
	if si == nil {
		return 0, errors.New("nextAvailableID is not initialized")
	}
	id := bigint.FromBytes(si)
	ret := int32(id.Int64())
	id.Add(id, intOne)
	si = bigint.ToPreallocatedBytes(id, si)
	return ret, d.PutStorageItem(m.ID, keyNextAvailableID, si)
}

func (m *Management) emitNotification(ic *interop.Context, name string, hash util.Uint160) {
	ne := state.NotificationEvent{
		ScriptHash: m.Hash,
		Name:       name,
		Item:       stackitem.NewArray([]stackitem.Item{addrToStackItem(&hash)}),
	}
	ic.Notifications = append(ic.Notifications, ne)
}

func checkScriptAndMethods(script []byte, methods []manifest.Method) error {
	l := len(script)
	offsets := bitfield.New(l)
	for i := range methods {
		if methods[i].Offset >= l {
			return errors.New("out of bounds method offset")
		}
		offsets.Set(methods[i].Offset)
	}
	return vm.IsScriptCorrect(script, offsets)
}
