package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxContractDescriptionLen is the maximum length for contract description.
	MaxContractDescriptionLen = 65536
	// MaxContractScriptSize is the maximum script size for a contract.
	MaxContractScriptSize = 1024 * 1024
	// MaxContractParametersNum is the maximum number of parameters for a contract.
	MaxContractParametersNum = 252
	// MaxContractStringLen is the maximum length for contract metadata strings.
	MaxContractStringLen = 252
)

var errGasLimitExceeded = errors.New("gas limit exceeded")

// storageFind finds stored key-value pair.
func storageFind(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	prefix := ic.VM.Estack().Pop().Bytes()
	siMap, err := ic.DAO.GetStorageItemsWithPrefix(stc.ID, prefix)
	if err != nil {
		return err
	}

	filteredMap := stackitem.NewMap()
	for k, v := range siMap {
		filteredMap.Add(stackitem.NewByteArray(append(prefix, []byte(k)...)), stackitem.NewByteArray(v.Value))
	}
	sort.Slice(filteredMap.Value().([]stackitem.MapElement), func(i, j int) bool {
		return bytes.Compare(filteredMap.Value().([]stackitem.MapElement)[i].Key.Value().([]byte),
			filteredMap.Value().([]stackitem.MapElement)[j].Key.Value().([]byte)) == -1
	})

	item := vm.NewMapIterator(filteredMap)
	ic.VM.Estack().PushVal(item)

	return nil
}

// getNefAndManifestFromVM pops NEF and manifest from the VM's evaluation stack,
// does a lot of checks and returns deserialized structures if succeeds.
func getNefAndManifestFromVM(v *vm.VM) (*nef.File, *manifest.Manifest, error) {
	// Always pop both elements.
	nefBytes := v.Estack().Pop().BytesOrNil()
	manifestBytes := v.Estack().Pop().BytesOrNil()

	if err := checkNonEmpty(nefBytes, math.MaxInt32); err != nil { // Upper limits are checked during NEF deserialization.
		return nil, nil, fmt.Errorf("invalid NEF file: %w", err)
	}
	if err := checkNonEmpty(manifestBytes, manifest.MaxManifestSize); err != nil {
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

// contractCreate creates a contract.
func contractCreate(ic *interop.Context) error {
	neff, manif, err := getNefAndManifestFromVM(ic.VM)
	if err != nil {
		return err
	}
	if neff == nil {
		return errors.New("no valid NEF provided")
	}
	if manif == nil {
		return errors.New("no valid manifest provided")
	}
	if ic.Tx == nil {
		return errors.New("no transaction provided")
	}
	h := state.CreateContractHash(ic.Tx.Sender(), neff.Script)
	contract, err := ic.DAO.GetContractState(h)
	if contract != nil && err == nil {
		return errors.New("contract already exists")
	}
	if !manif.IsValid(h) {
		return errors.New("failed to check contract script hash against manifest")
	}
	id, err := ic.DAO.GetAndUpdateNextContractID()
	if err != nil {
		return err
	}
	newcontract := &state.Contract{
		ID:       id,
		Hash:     h,
		Script:   neff.Script,
		Manifest: *manif,
	}
	if err := ic.DAO.PutContractState(newcontract); err != nil {
		return err
	}
	cs, err := contractToStackItem(newcontract)
	if err != nil {
		return fmt.Errorf("cannot convert contract to stack item: %w", err)
	}
	ic.VM.Estack().PushVal(cs)
	return callDeploy(ic, newcontract, false)
}

func checkNonEmpty(b []byte, max int) error {
	if b != nil {
		if l := len(b); l == 0 {
			return errors.New("empty")
		} else if l > max {
			return fmt.Errorf("len is %d (max %d)", l, max)
		}
	}
	return nil
}

// contractUpdate migrates a contract. This method assumes that Manifest and Script
// of the contract can be updated independently.
func contractUpdate(ic *interop.Context) error {
	neff, manif, err := getNefAndManifestFromVM(ic.VM)
	if err != nil {
		return err
	}
	if neff == nil && manif == nil {
		return errors.New("both NEF and manifest are nil")
	}
	contract, _ := ic.DAO.GetContractState(ic.VM.GetCurrentScriptHash())
	if contract == nil {
		return errors.New("contract doesn't exist")
	}
	// if NEF was provided, update the contract script
	if neff != nil {
		contract.Script = neff.Script
	}
	// if manifest was provided, update the contract manifest
	if manif != nil {
		contract.Manifest = *manif
		if !contract.Manifest.IsValid(contract.Hash) {
			return errors.New("failed to check contract script hash against new manifest")
		}
	}
	contract.UpdateCounter++

	if err := ic.DAO.PutContractState(contract); err != nil {
		return fmt.Errorf("failed to update contract: %w", err)
	}
	return callDeploy(ic, contract, true)
}

func callDeploy(ic *interop.Context, cs *state.Contract, isUpdate bool) error {
	md := cs.Manifest.ABI.GetMethod(manifest.MethodDeploy)
	if md != nil {
		return contract.CallExInternal(ic, cs, manifest.MethodDeploy,
			[]stackitem.Item{stackitem.NewBool(isUpdate)}, smartcontract.All, vm.EnsureIsEmpty)
	}
	return nil
}
