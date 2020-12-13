package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Management is contract-managing native contract.
type Management struct {
	interop.ContractMD
}

// StoragePrice is the price to pay for 1 byte of storage.
const StoragePrice = 100000

const (
	prefixContract = 8
)

var errGasLimitExceeded = errors.New("gas limit exceeded")
var keyNextAvailableID = []byte{15}

// makeContractKey creates a key from account script hash.
func makeContractKey(h util.Uint160) []byte {
	return makeUint160Key(prefixContract, h)
}

// newManagement creates new Management native contract.
func newManagement() *Management {
	var m = &Management{ContractMD: *interop.NewContractMD(nativenames.Management)}

	desc := newDescriptor("getContract", smartcontract.ArrayType,
		manifest.NewParameter("hash", smartcontract.Hash160Type))
	md := newMethodAndPrice(m.getContract, 1000000, smartcontract.ReadStates)
	m.AddMethod(md, desc)

	desc = newDescriptor("deploy", smartcontract.ArrayType,
		manifest.NewParameter("script", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType))
	md = newMethodAndPrice(m.deploy, 0, smartcontract.WriteStates)
	m.AddMethod(md, desc)

	desc = newDescriptor("update", smartcontract.VoidType,
		manifest.NewParameter("script", smartcontract.ByteArrayType),
		manifest.NewParameter("manifest", smartcontract.ByteArrayType))
	md = newMethodAndPrice(m.update, 0, smartcontract.WriteStates)
	m.AddMethod(md, desc)

	desc = newDescriptor("destroy", smartcontract.VoidType)
	md = newMethodAndPrice(m.destroy, 10000000, smartcontract.WriteStates)
	m.AddMethod(md, desc)

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
		panic(err)
	}
	return contractToStack(ctr)
}

// GetContract returns contract with given hash from given DAO.
func (m *Management) GetContract(d dao.DAO, hash util.Uint160) (*state.Contract, error) {
	contract := new(state.Contract)
	key := makeContractKey(hash)
	err := getSerializableFromDAO(m.ContractID, d, key, contract)
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
func getNefAndManifestFromItems(args []stackitem.Item, v *vm.VM) (*nef.File, *manifest.Manifest, error) {
	nefBytes, err := getLimitedSlice(args[0], math.MaxInt32) // Upper limits are checked during NEF deserialization.
	if err != nil {
		return nil, nil, fmt.Errorf("invalid NEF file: %w", err)
	}
	manifestBytes, err := getLimitedSlice(args[1], manifest.MaxManifestSize)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid manifest: %w", err)
	}

	if !v.AddGas(int64(StoragePrice * (len(nefBytes) + len(manifestBytes)))) {
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
		resManifest = new(manifest.Manifest)
		err := json.Unmarshal(manifestBytes, resManifest)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid manifest: %w", err)
		}
	}
	return resNef, resManifest, nil
}

// deploy is an implementation of public deploy method, it's run under
// VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) deploy(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	neff, manif, err := getNefAndManifestFromItems(args, ic.VM)
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
	callDeploy(ic, newcontract, false)
	return contractToStack(newcontract)

}

// Deploy creates contract's hash/ID and saves new contract into the given DAO.
// It doesn't run _deploy method.
func (m *Management) Deploy(d dao.DAO, sender util.Uint160, neff *nef.File, manif *manifest.Manifest) (*state.Contract, error) {
	h := state.CreateContractHash(sender, neff.Script)
	key := makeContractKey(h)
	si := d.GetStorageItem(m.ContractID, key)
	if si != nil {
		return nil, errors.New("contract already exists")
	}
	id, err := m.getNextContractID(d)
	if err != nil {
		return nil, err
	}
	if !manif.IsValid(h) {
		return nil, errors.New("invalid manifest for this contract")
	}
	newcontract := &state.Contract{
		ID:       id,
		Hash:     h,
		Script:   neff.Script,
		Manifest: *manif,
	}
	err = m.PutContractState(d, newcontract)
	if err != nil {
		return nil, err
	}
	return newcontract, nil
}

// update is an implementation of public update method, it's run under
// VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) update(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	neff, manif, err := getNefAndManifestFromItems(args, ic.VM)
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
	callDeploy(ic, contract, true)
	return stackitem.Null{}
}

// Update updates contract's script and/or manifest in the given DAO.
// It doesn't run _deploy method.
func (m *Management) Update(d dao.DAO, hash util.Uint160, neff *nef.File, manif *manifest.Manifest) (*state.Contract, error) {
	contract, err := m.GetContract(d, hash)
	if err != nil {
		return nil, errors.New("contract doesn't exist")
	}
	// if NEF was provided, update the contract script
	if neff != nil {
		contract.Script = neff.Script
	}
	// if manifest was provided, update the contract manifest
	if manif != nil {
		contract.Manifest = *manif
		if !contract.Manifest.IsValid(contract.Hash) {
			return nil, errors.New("invalid manifest for this contract")
		}
	}
	contract.UpdateCounter++
	err = m.PutContractState(d, contract)
	if err != nil {
		return nil, err
	}
	return contract, nil
}

// destroy is an implementation of destroy update method, it's run under
// VM protections, so it's OK for it to panic instead of returning errors.
func (m *Management) destroy(ic *interop.Context, sis []stackitem.Item) stackitem.Item {
	hash := ic.VM.GetCallingScriptHash()
	err := m.Destroy(ic.DAO, hash)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// Destroy drops given contract from DAO along with its storage.
func (m *Management) Destroy(d dao.DAO, hash util.Uint160) error {
	contract, err := m.GetContract(d, hash)
	if err != nil {
		return err
	}
	key := makeContractKey(hash)
	err = d.DeleteStorageItem(m.ContractID, key)
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
	return nil
}

func callDeploy(ic *interop.Context, cs *state.Contract, isUpdate bool) {
	md := cs.Manifest.ABI.GetMethod(manifest.MethodDeploy)
	if md != nil {
		err := contract.CallExInternal(ic, cs, manifest.MethodDeploy,
			[]stackitem.Item{stackitem.NewBool(isUpdate)}, smartcontract.All, vm.EnsureIsEmpty)
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
	if ic.Block.Index != 0 { // We're only deploying at 0 at the moment.
		return nil
	}

	for _, native := range ic.Natives {
		md := native.Metadata()

		cs := &state.Contract{
			ID:       md.ContractID,
			Hash:     md.Hash,
			Script:   md.Script,
			Manifest: md.Manifest,
		}
		err := m.PutContractState(ic.DAO, cs)
		if err != nil {
			return err
		}
		if err := native.Initialize(ic); err != nil {
			return fmt.Errorf("initializing %s native contract: %w", md.Name, err)
		}
	}

	return nil
}

// PostPersist implements Contract interface.
func (m *Management) PostPersist(_ *interop.Context) error {
	return nil
}

// Initialize implements Contract interface.
func (m *Management) Initialize(_ *interop.Context) error {
	return nil
}

// PutContractState saves given contract state into given DAO.
func (m *Management) PutContractState(d dao.DAO, cs *state.Contract) error {
	key := makeContractKey(cs.Hash)
	if err := putSerializableToDAO(m.ContractID, d, key, cs); err != nil {
		return err
	}
	if cs.UpdateCounter != 0 { // Update.
		return nil
	}
	return d.PutContractID(cs.ID, cs.Hash)
}

func (m *Management) getNextContractID(d dao.DAO) (int32, error) {
	var id = big.NewInt(1)
	si := d.GetStorageItem(m.ContractID, keyNextAvailableID)
	if si != nil {
		id = bigint.FromBytes(si.Value)
	} else {
		si = new(state.StorageItem)
		si.Value = make([]byte, 0, 2)
	}
	ret := int32(id.Int64())
	id.Add(id, big.NewInt(1))
	si.Value = bigint.ToPreallocatedBytes(id, si.Value)
	return ret, d.PutStorageItem(m.ContractID, keyNextAvailableID, si)
}
