package core

import (
	"errors"
	"fmt"
	"math"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/smartcontract/trigger"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	gherr "github.com/pkg/errors"
)

const (
	// MaxContractScriptSize is the maximum script size for a contract.
	MaxContractScriptSize = 1024 * 1024
	// MaxContractParametersNum is the maximum number of parameters for a contract.
	MaxContractParametersNum = 252
	// MaxContractStringLen is the maximum length for contract metadata strings.
	MaxContractStringLen = 252
	// MaxAssetNameLen is the maximum length of asset name.
	MaxAssetNameLen = 1024
	// MaxAssetPrecision is the maximum precision of asset.
	MaxAssetPrecision = 8
	// BlocksPerYear is a multiplier for asset renewal.
	BlocksPerYear = 2000000
	// DefaultAssetLifetime is the default lifetime of an asset (which differs
	// from assets created by register tx).
	DefaultAssetLifetime = 1 + BlocksPerYear
)

// txInOut is used to pushed one key-value pair from References() onto the stack.
type txInOut struct {
	in  transaction.Input
	out transaction.Output
}

// headerGetVersion returns version from the header.
func (ic *interopContext) headerGetVersion(v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.Version)
	return nil
}

// headerGetConsensusData returns consensus data from the header.
func (ic *interopContext) headerGetConsensusData(v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.ConsensusData)
	return nil
}

// headerGetMerkleRoot returns version from the header.
func (ic *interopContext) headerGetMerkleRoot(v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.MerkleRoot.BytesLE())
	return nil
}

// headerGetNextConsensus returns version from the header.
func (ic *interopContext) headerGetNextConsensus(v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.NextConsensus.BytesLE())
	return nil
}

// txGetAttributes returns current transaction attributes.
func (ic *interopContext) txGetAttributes(v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	if len(tx.Attributes) > vm.MaxArraySize {
		return errors.New("too many attributes")
	}
	attrs := make([]vm.StackItem, 0, len(tx.Attributes))
	for i := range tx.Attributes {
		attrs = append(attrs, vm.NewInteropItem(&tx.Attributes[i]))
	}
	v.Estack().PushVal(attrs)
	return nil
}

// txGetInputs returns current transaction inputs.
func (ic *interopContext) txGetInputs(v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	if len(tx.Inputs) > vm.MaxArraySize {
		return errors.New("too many inputs")
	}
	inputs := make([]vm.StackItem, 0, len(tx.Inputs))
	for i := range tx.Inputs {
		inputs = append(inputs, vm.NewInteropItem(&tx.Inputs[i]))
	}
	v.Estack().PushVal(inputs)
	return nil
}

// txGetOutputs returns current transaction outputs.
func (ic *interopContext) txGetOutputs(v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	if len(tx.Outputs) > vm.MaxArraySize {
		return errors.New("too many outputs")
	}
	outputs := make([]vm.StackItem, 0, len(tx.Outputs))
	for i := range tx.Outputs {
		outputs = append(outputs, vm.NewInteropItem(&tx.Outputs[i]))
	}
	v.Estack().PushVal(outputs)
	return nil
}

// txGetReferences returns current transaction references.
func (ic *interopContext) txGetReferences(v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return fmt.Errorf("type mismatch: %T is not a Transaction", txInterface)
	}
	refs := ic.bc.References(tx)
	if len(refs) > vm.MaxArraySize {
		return errors.New("too many references")
	}

	stackrefs := make([]vm.StackItem, 0, len(refs))
	for _, k := range tx.Inputs {
		tio := txInOut{k, *refs[k]}
		stackrefs = append(stackrefs, vm.NewInteropItem(tio))
	}
	v.Estack().PushVal(stackrefs)
	return nil
}

// txGetType returns current transaction type.
func (ic *interopContext) txGetType(v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	v.Estack().PushVal(int(tx.Type))
	return nil
}

// txGetUnspentCoins returns current transaction unspent coins.
func (ic *interopContext) txGetUnspentCoins(v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	ucs := ic.bc.GetUnspentCoinState(tx.Hash())
	if ucs == nil {
		return errors.New("no unspent coin state found")
	}
	v.Estack().PushVal(vm.NewInteropItem(ucs))
	return nil
}

// txGetWitnesses returns current transaction witnesses.
func (ic *interopContext) txGetWitnesses(v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	if len(tx.Scripts) > vm.MaxArraySize {
		return errors.New("too many outputs")
	}
	scripts := make([]vm.StackItem, 0, len(tx.Scripts))
	for i := range tx.Scripts {
		scripts = append(scripts, vm.NewInteropItem(&tx.Scripts[i]))
	}
	v.Estack().PushVal(scripts)
	return nil
}

// bcGetValidators returns validators.
func (ic *interopContext) bcGetValidators(v *vm.VM) error {
	validators, err := ic.bc.GetValidators()
	if err != nil {
		return err
	}
	v.Estack().PushVal(validators)
	return nil
}

// popInputFromVM returns transaction.Input from the first estack element.
func popInputFromVM(v *vm.VM) (*transaction.Input, error) {
	inInterface := v.Estack().Pop().Value()
	input, ok := inInterface.(*transaction.Input)
	if !ok {
		txio, ok := inInterface.(txInOut)
		if !ok {
			return nil, fmt.Errorf("type mismatch: %T is not an Input or txInOut", inInterface)
		}
		input = &txio.in
	}
	return input, nil
}

// inputGetHash returns hash from the given input.
func (ic *interopContext) inputGetHash(v *vm.VM) error {
	input, err := popInputFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(input.PrevHash.BytesBE())
	return nil
}

// inputGetIndex returns index from the given input.
func (ic *interopContext) inputGetIndex(v *vm.VM) error {
	input, err := popInputFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(input.PrevIndex)
	return nil
}

// popOutputFromVM returns transaction.Input from the first estack element.
func popOutputFromVM(v *vm.VM) (*transaction.Output, error) {
	outInterface := v.Estack().Pop().Value()
	output, ok := outInterface.(*transaction.Output)
	if !ok {
		txio, ok := outInterface.(txInOut)
		if !ok {
			return nil, fmt.Errorf("type mismatch: %T is not an Output or txInOut", outInterface)
		}
		output = &txio.out
	}
	return output, nil
}

// outputGetAssetId returns asset ID from the given output.
func (ic *interopContext) outputGetAssetID(v *vm.VM) error {
	output, err := popOutputFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(output.AssetID.BytesBE())
	return nil
}

// outputGetScriptHash returns scripthash from the given output.
func (ic *interopContext) outputGetScriptHash(v *vm.VM) error {
	output, err := popOutputFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(output.ScriptHash.BytesBE())
	return nil
}

// outputGetValue returns value (amount) from the given output.
func (ic *interopContext) outputGetValue(v *vm.VM) error {
	output, err := popOutputFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(int64(output.Amount))
	return nil
}

// attrGetData returns tx attribute data.
func (ic *interopContext) attrGetData(v *vm.VM) error {
	attrInterface := v.Estack().Pop().Value()
	attr, ok := attrInterface.(*transaction.Attribute)
	if !ok {
		return fmt.Errorf("%T is not an attribute", attr)
	}
	v.Estack().PushVal(attr.Data)
	return nil
}

// attrGetData returns tx attribute usage field.
func (ic *interopContext) attrGetUsage(v *vm.VM) error {
	attrInterface := v.Estack().Pop().Value()
	attr, ok := attrInterface.(*transaction.Attribute)
	if !ok {
		return fmt.Errorf("%T is not an attribute", attr)
	}
	v.Estack().PushVal(int(attr.Usage))
	return nil
}

// bcGetAccount returns or creates an account.
func (ic *interopContext) bcGetAccount(v *vm.VM) error {
	accbytes := v.Estack().Pop().Bytes()
	acchash, err := util.Uint160DecodeBytesBE(accbytes)
	if err != nil {
		return err
	}
	acc := ic.bc.GetAccountState(acchash)
	if acc == nil {
		acc = NewAccountState(acchash)
	}
	v.Estack().PushVal(vm.NewInteropItem(acc))
	return nil
}

// bcGetAsset returns an asset.
func (ic *interopContext) bcGetAsset(v *vm.VM) error {
	asbytes := v.Estack().Pop().Bytes()
	ashash, err := util.Uint256DecodeBytesBE(asbytes)
	if err != nil {
		return err
	}
	as := ic.bc.GetAssetState(ashash)
	if as == nil {
		return errors.New("asset not found")
	}
	v.Estack().PushVal(vm.NewInteropItem(as))
	return nil
}

// accountGetBalance returns balance for a given account.
func (ic *interopContext) accountGetBalance(v *vm.VM) error {
	accInterface := v.Estack().Pop().Value()
	acc, ok := accInterface.(*AccountState)
	if !ok {
		return fmt.Errorf("%T is not an account state", acc)
	}
	asbytes := v.Estack().Pop().Bytes()
	ashash, err := util.Uint256DecodeBytesBE(asbytes)
	if err != nil {
		return err
	}
	balance, ok := acc.GetBalanceValues()[ashash]
	if !ok {
		balance = util.Fixed8(0)
	}
	v.Estack().PushVal(int64(balance))
	return nil
}

// accountGetScriptHash returns script hash of a given account.
func (ic *interopContext) accountGetScriptHash(v *vm.VM) error {
	accInterface := v.Estack().Pop().Value()
	acc, ok := accInterface.(*AccountState)
	if !ok {
		return fmt.Errorf("%T is not an account state", acc)
	}
	v.Estack().PushVal(acc.ScriptHash.BytesBE())
	return nil
}

// accountGetVotes returns votes of a given account.
func (ic *interopContext) accountGetVotes(v *vm.VM) error {
	accInterface := v.Estack().Pop().Value()
	acc, ok := accInterface.(*AccountState)
	if !ok {
		return fmt.Errorf("%T is not an account state", acc)
	}
	if len(acc.Votes) > vm.MaxArraySize {
		return errors.New("too many votes")
	}
	votes := make([]vm.StackItem, 0, len(acc.Votes))
	for _, key := range acc.Votes {
		votes = append(votes, vm.NewByteArrayItem(key.Bytes()))
	}
	v.Estack().PushVal(votes)
	return nil
}

// accountIsStandard checks whether given account is standard.
func (ic *interopContext) accountIsStandard(v *vm.VM) error {
	accbytes := v.Estack().Pop().Bytes()
	acchash, err := util.Uint160DecodeBytesBE(accbytes)
	if err != nil {
		return err
	}
	contract := ic.bc.GetContractState(acchash)
	res := contract == nil || vm.IsStandardContract(contract.Script)
	v.Estack().PushVal(res)
	return nil
}

/*
// storageFind finds stored key-value pair.
func (ic *interopContext) storageFind(v *vm.VM) error {
	stcInterface := v.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	err := ic.checkStorageContext(stc)
	if err != nil {
		return err
	}
	prefix := string(v.Estack().Pop().Bytes())
	siMap, err := ic.bc.GetStorageItems(stc.ScriptHash)
	if err != nil {
		return err
	}
	for k, v := range siMap {
		if strings.HasPrefix(k, prefix) {
			_ = v
			panic("TODO")
		}
	}

	return nil
}
*/
// createContractStateFromVM pops all contract state elements from the VM
// evaluation stack, does a lot of checks and returns ContractState if it
// succeeds.
func (ic *interopContext) createContractStateFromVM(v *vm.VM) (*ContractState, error) {
	if ic.trigger != trigger.Application {
		return nil, errors.New("can't create contract when not triggered by an application")
	}
	script := v.Estack().Pop().Bytes()
	if len(script) > MaxContractScriptSize {
		return nil, errors.New("the script is too big")
	}
	paramBytes := v.Estack().Pop().Bytes()
	if len(paramBytes) > MaxContractParametersNum {
		return nil, errors.New("too many parameters for a script")
	}
	paramList := make([]smartcontract.ParamType, len(paramBytes))
	for k, v := range paramBytes {
		paramList[k] = smartcontract.ParamType(v)
	}
	retType := smartcontract.ParamType(v.Estack().Pop().BigInt().Int64())
	properties := smartcontract.PropertyState(v.Estack().Pop().BigInt().Int64())
	name := v.Estack().Pop().Bytes()
	if len(name) > MaxContractStringLen {
		return nil, errors.New("too big name")
	}
	version := v.Estack().Pop().Bytes()
	if len(version) > MaxContractStringLen {
		return nil, errors.New("too big version")
	}
	author := v.Estack().Pop().Bytes()
	if len(author) > MaxContractStringLen {
		return nil, errors.New("too big author")
	}
	email := v.Estack().Pop().Bytes()
	if len(email) > MaxContractStringLen {
		return nil, errors.New("too big email")
	}
	desc := v.Estack().Pop().Bytes()
	if len(desc) > MaxContractStringLen {
		return nil, errors.New("too big description")
	}
	contract := &ContractState{
		Script:      script,
		ParamList:   paramList,
		ReturnType:  retType,
		Properties:  properties,
		Name:        string(name),
		CodeVersion: string(version),
		Author:      string(author),
		Email:       string(email),
		Description: string(desc),
	}
	return contract, nil
}

// contractCreate creates a contract.
func (ic *interopContext) contractCreate(v *vm.VM) error {
	newcontract, err := ic.createContractStateFromVM(v)
	if err != nil {
		return nil
	}
	contract := ic.bc.GetContractState(newcontract.ScriptHash())
	if contract == nil {
		contract = newcontract
		err := putContractStateIntoStore(ic.mem, contract)
		if err != nil {
			return err
		}
	}
	v.Estack().PushVal(vm.NewInteropItem(contract))
	return nil
}

// contractGetScript returns a script associated with a contract.
func (ic *interopContext) contractGetScript(v *vm.VM) error {
	csInterface := v.Estack().Pop().Value()
	cs, ok := csInterface.(*ContractState)
	if !ok {
		return fmt.Errorf("%T is not a contract state", cs)
	}
	v.Estack().PushVal(cs.Script)
	return nil
}

// contractIsPayable returns whether contract is payable.
func (ic *interopContext) contractIsPayable(v *vm.VM) error {
	csInterface := v.Estack().Pop().Value()
	cs, ok := csInterface.(*ContractState)
	if !ok {
		return fmt.Errorf("%T is not a contract state", cs)
	}
	v.Estack().PushVal(cs.IsPayable())
	return nil
}

// contractMigrate migrates a contract.
func (ic *interopContext) contractMigrate(v *vm.VM) error {
	newcontract, err := ic.createContractStateFromVM(v)
	if err != nil {
		return nil
	}
	contract := ic.bc.GetContractState(newcontract.ScriptHash())
	if contract == nil {
		contract = newcontract
		err := putContractStateIntoStore(ic.mem, contract)
		if err != nil {
			return err
		}
		if contract.HasStorage() {
			hash := getContextScriptHash(v, 0)
			siMap, err := ic.bc.GetStorageItems(hash)
			if err != nil {
				return err
			}
			for k, v := range siMap {
				v.IsConst = false
				_ = putStorageItemIntoStore(ic.mem, hash, []byte(k), v)
			}
		}
	}
	v.Estack().PushVal(vm.NewInteropItem(contract))
	return ic.contractDestroy(v)
}

// assetCreate creates an asset.
func (ic *interopContext) assetCreate(v *vm.VM) error {
	if ic.trigger != trigger.Application {
		return errors.New("can't create asset when not triggered by an application")
	}
	atype := transaction.AssetType(v.Estack().Pop().BigInt().Int64())
	switch atype {
	case transaction.Currency, transaction.Share, transaction.Invoice, transaction.Token:
		// ok
	default:
		return fmt.Errorf("wrong asset type: %x", atype)
	}
	name := string(v.Estack().Pop().Bytes())
	if len(name) > MaxAssetNameLen {
		return errors.New("too big name")
	}
	amount := util.Fixed8(v.Estack().Pop().BigInt().Int64())
	if amount == util.Fixed8(0) {
		return errors.New("asset amount can't be zero")
	}
	if amount < -util.Satoshi() {
		return errors.New("asset amount can't be negative (except special -Satoshi value")
	}
	if atype == transaction.Invoice && amount != -util.Satoshi() {
		return errors.New("invoice assets can only have -Satoshi amount")
	}
	precision := byte(v.Estack().Pop().BigInt().Int64())
	if precision > MaxAssetPrecision {
		return fmt.Errorf("can't have asset precision of more than %d", MaxAssetPrecision)
	}
	if atype == transaction.Share && precision != 0 {
		return errors.New("share assets can only have zero precision")
	}
	if amount != -util.Satoshi() && (int64(amount)%int64(math.Pow10(int(MaxAssetPrecision-precision))) != 0) {
		return errors.New("given asset amount has fractional component")
	}
	owner := &keys.PublicKey{}
	err := owner.DecodeBytes(v.Estack().Pop().Bytes())
	if err != nil {
		return gherr.Wrap(err, "failed to get owner key")
	}
	if owner.IsInfinity() {
		return errors.New("can't have infinity as an owner key")
	}
	witnessOk, err := ic.checkKeyedWitness(owner)
	if err != nil {
		return err
	}
	if !witnessOk {
		return errors.New("witness check didn't succeed")
	}
	admin, err := util.Uint160DecodeBytesBE(v.Estack().Pop().Bytes())
	if err != nil {
		return gherr.Wrap(err, "failed to get admin")
	}
	issuer, err := util.Uint160DecodeBytesBE(v.Estack().Pop().Bytes())
	if err != nil {
		return gherr.Wrap(err, "failed to get issuer")
	}
	asset := &AssetState{
		ID:         ic.tx.Hash(),
		AssetType:  atype,
		Name:       name,
		Amount:     amount,
		Precision:  precision,
		Owner:      owner,
		Admin:      admin,
		Issuer:     issuer,
		Expiration: ic.bc.BlockHeight() + DefaultAssetLifetime,
	}
	err = putAssetStateIntoStore(ic.mem, asset)
	if err != nil {
		return gherr.Wrap(err, "failed to store asset")
	}
	v.Estack().PushVal(vm.NewInteropItem(asset))
	return nil
}

// assetGetAdmin returns asset admin.
func (ic *interopContext) assetGetAdmin(v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(as.Admin.BytesBE())
	return nil
}

// assetGetAmount returns the overall amount of asset available.
func (ic *interopContext) assetGetAmount(v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(int64(as.Amount))
	return nil
}

// assetGetAssetId returns the id of an asset.
func (ic *interopContext) assetGetAssetID(v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(as.ID.BytesBE())
	return nil
}

// assetGetAssetType returns type of an asset.
func (ic *interopContext) assetGetAssetType(v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(int(as.AssetType))
	return nil
}

// assetGetAvailable returns available (not yet issued) amount of asset.
func (ic *interopContext) assetGetAvailable(v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(int(as.Available))
	return nil
}

// assetGetIssuer returns issuer of an asset.
func (ic *interopContext) assetGetIssuer(v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(as.Issuer.BytesBE())
	return nil
}

// assetGetOwner returns owner of an asset.
func (ic *interopContext) assetGetOwner(v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(as.Owner.Bytes())
	return nil
}

// assetGetPrecision returns precision used to measure this asset.
func (ic *interopContext) assetGetPrecision(v *vm.VM) error {
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	v.Estack().PushVal(int(as.Precision))
	return nil
}

// assetRenew updates asset expiration date.
func (ic *interopContext) assetRenew(v *vm.VM) error {
	if ic.trigger != trigger.Application {
		return errors.New("can't create asset when not triggered by an application")
	}
	asInterface := v.Estack().Pop().Value()
	as, ok := asInterface.(*AssetState)
	if !ok {
		return fmt.Errorf("%T is not an asset state", as)
	}
	years := byte(v.Estack().Pop().BigInt().Int64())
	// Not sure why C# code regets an asset from the Store, but we also do it.
	asset := ic.bc.GetAssetState(as.ID)
	if asset == nil {
		return errors.New("can't renew non-existent asset")
	}
	if asset.Expiration < ic.bc.BlockHeight()+1 {
		asset.Expiration = ic.bc.BlockHeight() + 1
	}
	expiration := uint64(asset.Expiration) + uint64(years)*BlocksPerYear
	if expiration > math.MaxUint32 {
		expiration = math.MaxUint32
	}
	asset.Expiration = uint32(expiration)
	err := putAssetStateIntoStore(ic.mem, asset)
	if err != nil {
		return gherr.Wrap(err, "failed to store asset")
	}
	v.Estack().PushVal(expiration)
	return nil
}

// runtimeSerialize serializes top stack item into a ByteArray.
func (ic *interopContext) runtimeSerialize(v *vm.VM) error {
	return vm.RuntimeSerialize(v)
}

// runtimeDeserialize deserializes ByteArray from a stack into an item.
func (ic *interopContext) runtimeDeserialize(v *vm.VM) error {
	return vm.RuntimeDeserialize(v)
}
