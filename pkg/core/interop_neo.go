package core

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/mr-tron/base58"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
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

// createContractStateFromVM pops all contract state elements from the VM
// evaluation stack, does a lot of checks and returns Contract if it
// succeeds.
func createContractStateFromVM(ic *interop.Context) (*state.Contract, error) {
	script := ic.VM.Estack().Pop().Bytes()
	if len(script) > MaxContractScriptSize {
		return nil, errors.New("the script is too big")
	}
	manifestBytes := ic.VM.Estack().Pop().Bytes()
	if len(manifestBytes) > manifest.MaxManifestSize {
		return nil, errors.New("manifest is too big")
	}
	if !ic.VM.AddGas(int64(StoragePrice * (len(script) + len(manifestBytes)))) {
		return nil, errGasLimitExceeded
	}
	var m manifest.Manifest
	err := json.Unmarshal(manifestBytes, &m)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve manifest from stack: %w", err)
	}
	return &state.Contract{
		Script:   script,
		Manifest: m,
	}, nil
}

// contractCreate creates a contract.
func contractCreate(ic *interop.Context) error {
	newcontract, err := createContractStateFromVM(ic)
	if err != nil {
		return err
	}
	contract, err := ic.DAO.GetContractState(newcontract.ScriptHash())
	if contract != nil && err == nil {
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
	contract, _ := ic.DAO.GetContractState(ic.VM.GetCurrentScriptHash())
	if contract == nil {
		return errors.New("contract doesn't exist")
	}
	script := ic.VM.Estack().Pop().BytesOrNil()
	manifestBytes := ic.VM.Estack().Pop().BytesOrNil()
	if script == nil && manifestBytes == nil {
		return errors.New("both script and manifest are nil")
	}
	if err := checkNonEmpty(script, MaxContractScriptSize); err != nil {
		return fmt.Errorf("invalid script size: %w", err)
	}
	if err := checkNonEmpty(manifestBytes, manifest.MaxManifestSize); err != nil {
		return fmt.Errorf("invalid manifest size: %w", err)
	}
	if !ic.VM.AddGas(int64(StoragePrice * (len(script) + len(manifestBytes)))) {
		return errGasLimitExceeded
	}
	// if script was provided, update the old contract script and Manifest.ABI hash
	if script != nil {
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
		if err := ic.DAO.PutContractState(contract); err != nil {
			return fmt.Errorf("failed to update script: %w", err)
		}
		if err := ic.DAO.DeleteContractState(oldHash); err != nil {
			return fmt.Errorf("failed to update script: %w", err)
		}
	}
	// if manifest was provided, update the old contract manifest and check associated
	// storage items if needed
	if manifestBytes != nil {
		var newManifest manifest.Manifest
		err := json.Unmarshal(manifestBytes, &newManifest)
		if err != nil {
			return fmt.Errorf("unable to retrieve manifest from stack: %w", err)
		}
		// we don't have to perform `GetContractState` one more time as it's already up-to-date
		contract.Manifest = newManifest
		if !contract.Manifest.IsValid(contract.ScriptHash()) {
			return errors.New("failed to check contract script hash against new manifest")
		}
		if err := ic.DAO.PutContractState(contract); err != nil {
			return fmt.Errorf("failed to update manifest: %w", err)
		}
	}

	return callDeploy(ic, contract, true)
}

func callDeploy(ic *interop.Context, cs *state.Contract, isUpdate bool) error {
	md := cs.Manifest.ABI.GetMethod(manifest.MethodDeploy)
	if md != nil {
		return contract.CallExInternal(ic, cs, manifest.MethodDeploy,
			[]stackitem.Item{stackitem.NewBool(isUpdate)}, smartcontract.All, vm.EnsureIsEmpty, nil)
	}
	return nil
}

// runtimeSerialize serializes top stack item into a ByteArray.
func runtimeSerialize(ic *interop.Context) error {
	return vm.RuntimeSerialize(ic.VM)
}

// runtimeDeserialize deserializes ByteArray from a stack into an item.
func runtimeDeserialize(ic *interop.Context) error {
	return vm.RuntimeDeserialize(ic.VM)
}

// runtimeEncodeBase64 encodes top stack item into a base64 string.
func runtimeEncodeBase64(ic *interop.Context) error {
	src := ic.VM.Estack().Pop().Bytes()
	result := base64.StdEncoding.EncodeToString(src)
	ic.VM.Estack().PushVal([]byte(result))
	return nil
}

// runtimeDecodeBase64 decodes top stack item from base64 string to byte array.
func runtimeDecodeBase64(ic *interop.Context) error {
	src := ic.VM.Estack().Pop().String()
	result, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(result)
	return nil
}

// runtimeEncodeBase58 encodes top stack item into a base58 string.
func runtimeEncodeBase58(ic *interop.Context) error {
	src := ic.VM.Estack().Pop().Bytes()
	result := base58.Encode(src)
	ic.VM.Estack().PushVal([]byte(result))
	return nil
}

// runtimeDecodeBase58 decodes top stack item from base58 string to byte array.
func runtimeDecodeBase58(ic *interop.Context) error {
	src := ic.VM.Estack().Pop().String()
	result, err := base58.Decode(src)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(result)
	return nil
}
