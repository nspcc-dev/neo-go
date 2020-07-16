package core

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
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
func storageFind(ic *interop.Context, v *vm.VM) error {
	stcInterface := v.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	prefix := v.Estack().Pop().Bytes()
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
	v.Estack().PushVal(item)

	return nil
}

// createContractStateFromVM pops all contract state elements from the VM
// evaluation stack, does a lot of checks and returns Contract if it
// succeeds.
func createContractStateFromVM(ic *interop.Context, v *vm.VM) (*state.Contract, error) {
	script := v.Estack().Pop().Bytes()
	if len(script) > MaxContractScriptSize {
		return nil, errors.New("the script is too big")
	}
	manifestBytes := v.Estack().Pop().Bytes()
	if len(manifestBytes) > manifest.MaxManifestSize {
		return nil, errors.New("manifest is too big")
	}
	if !v.AddGas(int64(StoragePrice * (len(script) + len(manifestBytes)))) {
		return nil, errGasLimitExceeded
	}
	var m manifest.Manifest
	err := m.UnmarshalJSON(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve manifest from stack: %v", err)
	}
	return &state.Contract{
		Script:   script,
		Manifest: m,
	}, nil
}

// contractCreate creates a contract.
func contractCreate(ic *interop.Context, v *vm.VM) error {
	newcontract, err := createContractStateFromVM(ic, v)
	if err != nil {
		return err
	}
	contract, err := ic.DAO.GetContractState(newcontract.ScriptHash())
	if contract != nil {
		return errors.New("contract already exists")
	}
	id, err := ic.DAO.GetAndUpdateNextContractID()
	if err != nil {
		return err
	}
	newcontract.ID = id
	if !newcontract.Manifest.IsValid(newcontract.ScriptHash()) {
		return errors.New("failed to check contract script hash against manifest")
	}
	if err := ic.DAO.PutContractState(newcontract); err != nil {
		return err
	}
	cs, err := contractToStackItem(newcontract)
	if err != nil {
		return fmt.Errorf("cannot convert contract to stack item: %v", err)
	}
	v.Estack().PushVal(cs)
	return nil
}

// contractUpdate migrates a contract. This method assumes that Manifest and Script
// of the contract can be updated independently.
func contractUpdate(ic *interop.Context, v *vm.VM) error {
	contract, _ := ic.DAO.GetContractState(v.GetCurrentScriptHash())
	if contract == nil {
		return errors.New("contract doesn't exist")
	}
	script := v.Estack().Pop().Bytes()
	if len(script) > MaxContractScriptSize {
		return errors.New("the script is too big")
	}
	manifestBytes := v.Estack().Pop().Bytes()
	if len(manifestBytes) > manifest.MaxManifestSize {
		return errors.New("manifest is too big")
	}
	if !v.AddGas(int64(StoragePrice * (len(script) + len(manifestBytes)))) {
		return errGasLimitExceeded
	}
	// if script was provided, update the old contract script and Manifest.ABI hash
	if l := len(script); l > 0 {
		if l > MaxContractScriptSize {
			return errors.New("invalid script len")
		}
		newHash := hash.Hash160(script)
		if newHash.Equals(contract.ScriptHash()) {
			return errors.New("the script is the same")
		} else if _, err := ic.DAO.GetContractState(newHash); err == nil {
			return errors.New("contract already exists")
		}
		oldHash := contract.ScriptHash()
		// re-write existing contract variable, as we need it to be up-to-date during manifest update
		contract = &state.Contract{
			ID:       contract.ID,
			Script:   script,
			Manifest: contract.Manifest,
		}
		contract.Manifest.ABI.Hash = newHash
		if err := ic.DAO.PutContractState(contract); err != nil {
			return fmt.Errorf("failed to update script: %v", err)
		}
		if err := ic.DAO.DeleteContractState(oldHash); err != nil {
			return fmt.Errorf("failed to update script: %v", err)
		}
	}
	// if manifest was provided, update the old contract manifest and check associated
	// storage items if needed
	if len(manifestBytes) > 0 {
		var newManifest manifest.Manifest
		err := newManifest.UnmarshalJSON(manifestBytes)
		if err != nil {
			return fmt.Errorf("unable to retrieve manifest from stack: %v", err)
		}
		// we don't have to perform `GetContractState` one more time as it's already up-to-date
		contract.Manifest = newManifest
		if !contract.Manifest.IsValid(contract.ScriptHash()) {
			return errors.New("failed to check contract script hash against new manifest")
		}
		if !contract.HasStorage() {
			siMap, err := ic.DAO.GetStorageItems(contract.ID)
			if err != nil {
				return fmt.Errorf("failed to update manifest: %v", err)
			}
			if len(siMap) != 0 {
				return errors.New("old contract shouldn't have storage")
			}
		}
		if err := ic.DAO.PutContractState(contract); err != nil {
			return fmt.Errorf("failed to update manifest: %v", err)
		}
	}
	return nil
}

// runtimeSerialize serializes top stack item into a ByteArray.
func runtimeSerialize(_ *interop.Context, v *vm.VM) error {
	return vm.RuntimeSerialize(v)
}

// runtimeDeserialize deserializes ByteArray from a stack into an item.
func runtimeDeserialize(_ *interop.Context, v *vm.VM) error {
	return vm.RuntimeDeserialize(v)
}
