package core

/*
  Interops are designed to run under VM's execute() panic protection, so it's OK
  for them to do things like
          smth := v.Estack().Pop().Bytes()
  even though technically Pop() can return a nil pointer.
*/

import (
	"sort"

	"github.com/CityOfZion/neo-go/pkg/core/state"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

type interopContext struct {
	bc            Blockchainer
	trigger       byte
	block         *Block
	tx            *transaction.Transaction
	dao           *cachedDao
	notifications []state.NotificationEvent
}

func newInteropContext(trigger byte, bc Blockchainer, s storage.Store, block *Block, tx *transaction.Transaction) *interopContext {
	dao := newCachedDao(s)
	nes := make([]state.NotificationEvent, 0)
	return &interopContext{bc, trigger, block, tx, dao, nes}
}

// interopedFunction binds function name, id with the function itself and price,
// it's supposed to be inited once for all interopContexts, so it doesn't use
// vm.InteropFuncPrice directly.
type interopedFunction struct {
	ID    uint32
	Name  string
	Func  func(*interopContext, *vm.VM) error
	Price int
}

// getSystemInterop returns matching interop function from the System namespace
// for a given id in the current context.
func (ic *interopContext) getSystemInterop(id uint32) *vm.InteropFuncPrice {
	return ic.getInteropFromSlice(id, systemInterops)
}

// getNeoInterop returns matching interop function from the Neo and AntShares
// namespaces for a given id in the current context.
func (ic *interopContext) getNeoInterop(id uint32) *vm.InteropFuncPrice {
	return ic.getInteropFromSlice(id, neoInterops)
}

// getInteropFromSlice returns matching interop function from the given slice of
// interop functions in the current context.
func (ic *interopContext) getInteropFromSlice(id uint32, slice []interopedFunction) *vm.InteropFuncPrice {
	n := sort.Search(len(slice), func(i int) bool {
		return slice[i].ID >= id
	})
	if n < len(slice) && slice[n].ID == id {
		// Currying, yay!
		return &vm.InteropFuncPrice{Func: func(v *vm.VM) error {
			return slice[n].Func(ic, v)
		}, Price: slice[n].Price}
	}
	return nil
}

// All lists are sorted, keep 'em this way, please.
var systemInterops = []interopedFunction{
	{Name: "System.Block.GetTransaction", Func: (*interopContext).blockGetTransaction, Price: 1},
	{Name: "System.Block.GetTransactionCount", Func: (*interopContext).blockGetTransactionCount, Price: 1},
	{Name: "System.Block.GetTransactions", Func: (*interopContext).blockGetTransactions, Price: 1},
	{Name: "System.Blockchain.GetBlock", Func: (*interopContext).bcGetBlock, Price: 200},
	{Name: "System.Blockchain.GetContract", Func: (*interopContext).bcGetContract, Price: 100},
	{Name: "System.Blockchain.GetHeader", Func: (*interopContext).bcGetHeader, Price: 100},
	{Name: "System.Blockchain.GetHeight", Func: (*interopContext).bcGetHeight, Price: 1},
	{Name: "System.Blockchain.GetTransaction", Func: (*interopContext).bcGetTransaction, Price: 200},
	{Name: "System.Blockchain.GetTransactionHeight", Func: (*interopContext).bcGetTransactionHeight, Price: 100},
	{Name: "System.Contract.Destroy", Func: (*interopContext).contractDestroy, Price: 1},
	{Name: "System.Contract.GetStorageContext", Func: (*interopContext).contractGetStorageContext, Price: 1},
	{Name: "System.ExecutionEngine.GetCallingScriptHash", Func: (*interopContext).engineGetCallingScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetEntryScriptHash", Func: (*interopContext).engineGetEntryScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetExecutingScriptHash", Func: (*interopContext).engineGetExecutingScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetScriptContainer", Func: (*interopContext).engineGetScriptContainer, Price: 1},
	{Name: "System.Header.GetHash", Func: (*interopContext).headerGetHash, Price: 1},
	{Name: "System.Header.GetIndex", Func: (*interopContext).headerGetIndex, Price: 1},
	{Name: "System.Header.GetPrevHash", Func: (*interopContext).headerGetPrevHash, Price: 1},
	{Name: "System.Header.GetTimestamp", Func: (*interopContext).headerGetTimestamp, Price: 1},
	{Name: "System.Runtime.CheckWitness", Func: (*interopContext).runtimeCheckWitness, Price: 200},
	{Name: "System.Runtime.Deserialize", Func: (*interopContext).runtimeDeserialize, Price: 1},
	{Name: "System.Runtime.GetTime", Func: (*interopContext).runtimeGetTime, Price: 1},
	{Name: "System.Runtime.GetTrigger", Func: (*interopContext).runtimeGetTrigger, Price: 1},
	{Name: "System.Runtime.Log", Func: (*interopContext).runtimeLog, Price: 1},
	{Name: "System.Runtime.Notify", Func: (*interopContext).runtimeNotify, Price: 1},
	{Name: "System.Runtime.Platform", Func: (*interopContext).runtimePlatform, Price: 1},
	{Name: "System.Runtime.Serialize", Func: (*interopContext).runtimeSerialize, Price: 1},
	{Name: "System.Storage.Delete", Func: (*interopContext).storageDelete, Price: 100},
	{Name: "System.Storage.Get", Func: (*interopContext).storageGet, Price: 100},
	{Name: "System.Storage.GetContext", Func: (*interopContext).storageGetContext, Price: 1},
	{Name: "System.Storage.GetReadOnlyContext", Func: (*interopContext).storageGetReadOnlyContext, Price: 1},
	{Name: "System.Storage.Put", Func: (*interopContext).storagePut, Price: 0}, // These don't have static price in C# code.
	{Name: "System.Storage.PutEx", Func: (*interopContext).storagePutEx, Price: 0},
	{Name: "System.StorageContext.AsReadOnly", Func: (*interopContext).storageContextAsReadOnly, Price: 1},
	{Name: "System.Transaction.GetHash", Func: (*interopContext).txGetHash, Price: 1},
}

var neoInterops = []interopedFunction{
	{Name: "Neo.Account.GetBalance", Func: (*interopContext).accountGetBalance, Price: 1},
	{Name: "Neo.Account.GetScriptHash", Func: (*interopContext).accountGetScriptHash, Price: 1},
	{Name: "Neo.Account.GetVotes", Func: (*interopContext).accountGetVotes, Price: 1},
	{Name: "Neo.Account.IsStandard", Func: (*interopContext).accountIsStandard, Price: 100},
	{Name: "Neo.Asset.Create", Func: (*interopContext).assetCreate, Price: 0},
	{Name: "Neo.Asset.GetAdmin", Func: (*interopContext).assetGetAdmin, Price: 1},
	{Name: "Neo.Asset.GetAmount", Func: (*interopContext).assetGetAmount, Price: 1},
	{Name: "Neo.Asset.GetAssetId", Func: (*interopContext).assetGetAssetID, Price: 1},
	{Name: "Neo.Asset.GetAssetType", Func: (*interopContext).assetGetAssetType, Price: 1},
	{Name: "Neo.Asset.GetAvailable", Func: (*interopContext).assetGetAvailable, Price: 1},
	{Name: "Neo.Asset.GetIssuer", Func: (*interopContext).assetGetIssuer, Price: 1},
	{Name: "Neo.Asset.GetOwner", Func: (*interopContext).assetGetOwner, Price: 1},
	{Name: "Neo.Asset.GetPrecision", Func: (*interopContext).assetGetPrecision, Price: 1},
	{Name: "Neo.Asset.Renew", Func: (*interopContext).assetRenew, Price: 0},
	{Name: "Neo.Attribute.GetData", Func: (*interopContext).attrGetData, Price: 1},
	{Name: "Neo.Attribute.GetUsage", Func: (*interopContext).attrGetUsage, Price: 1},
	{Name: "Neo.Block.GetTransaction", Func: (*interopContext).blockGetTransaction, Price: 1},
	{Name: "Neo.Block.GetTransactionCount", Func: (*interopContext).blockGetTransactionCount, Price: 1},
	{Name: "Neo.Block.GetTransactions", Func: (*interopContext).blockGetTransactions, Price: 1},
	{Name: "Neo.Blockchain.GetAccount", Func: (*interopContext).bcGetAccount, Price: 100},
	{Name: "Neo.Blockchain.GetAsset", Func: (*interopContext).bcGetAsset, Price: 100},
	{Name: "Neo.Blockchain.GetBlock", Func: (*interopContext).bcGetBlock, Price: 200},
	{Name: "Neo.Blockchain.GetContract", Func: (*interopContext).bcGetContract, Price: 100},
	{Name: "Neo.Blockchain.GetHeader", Func: (*interopContext).bcGetHeader, Price: 100},
	{Name: "Neo.Blockchain.GetHeight", Func: (*interopContext).bcGetHeight, Price: 1},
	{Name: "Neo.Blockchain.GetTransaction", Func: (*interopContext).bcGetTransaction, Price: 100},
	{Name: "Neo.Blockchain.GetTransactionHeight", Func: (*interopContext).bcGetTransactionHeight, Price: 100},
	{Name: "Neo.Blockchain.GetValidators", Func: (*interopContext).bcGetValidators, Price: 200},
	{Name: "Neo.Contract.Create", Func: (*interopContext).contractCreate, Price: 0},
	{Name: "Neo.Contract.Destroy", Func: (*interopContext).contractDestroy, Price: 1},
	{Name: "Neo.Contract.GetScript", Func: (*interopContext).contractGetScript, Price: 1},
	{Name: "Neo.Contract.GetStorageContext", Func: (*interopContext).contractGetStorageContext, Price: 1},
	{Name: "Neo.Contract.IsPayable", Func: (*interopContext).contractIsPayable, Price: 1},
	{Name: "Neo.Contract.Migrate", Func: (*interopContext).contractMigrate, Price: 0},
	{Name: "Neo.Header.GetConsensusData", Func: (*interopContext).headerGetConsensusData, Price: 1},
	{Name: "Neo.Header.GetHash", Func: (*interopContext).headerGetHash, Price: 1},
	{Name: "Neo.Header.GetIndex", Func: (*interopContext).headerGetIndex, Price: 1},
	{Name: "Neo.Header.GetMerkleRoot", Func: (*interopContext).headerGetMerkleRoot, Price: 1},
	{Name: "Neo.Header.GetNextConsensus", Func: (*interopContext).headerGetNextConsensus, Price: 1},
	{Name: "Neo.Header.GetPrevHash", Func: (*interopContext).headerGetPrevHash, Price: 1},
	{Name: "Neo.Header.GetTimestamp", Func: (*interopContext).headerGetTimestamp, Price: 1},
	{Name: "Neo.Header.GetVersion", Func: (*interopContext).headerGetVersion, Price: 1},
	{Name: "Neo.Input.GetHash", Func: (*interopContext).inputGetHash, Price: 1},
	{Name: "Neo.Input.GetIndex", Func: (*interopContext).inputGetIndex, Price: 1},
	{Name: "Neo.InvocationTransaction.GetScript", Func: (*interopContext).invocationTxGetScript, Price: 1},
	{Name: "Neo.Output.GetAssetId", Func: (*interopContext).outputGetAssetID, Price: 1},
	{Name: "Neo.Output.GetScriptHash", Func: (*interopContext).outputGetScriptHash, Price: 1},
	{Name: "Neo.Output.GetValue", Func: (*interopContext).outputGetValue, Price: 1},
	{Name: "Neo.Runtime.CheckWitness", Func: (*interopContext).runtimeCheckWitness, Price: 200},
	{Name: "Neo.Runtime.GetTime", Func: (*interopContext).runtimeGetTime, Price: 1},
	{Name: "Neo.Runtime.GetTrigger", Func: (*interopContext).runtimeGetTrigger, Price: 1},
	{Name: "Neo.Runtime.Log", Func: (*interopContext).runtimeLog, Price: 1},
	{Name: "Neo.Runtime.Notify", Func: (*interopContext).runtimeNotify, Price: 1},
	{Name: "Neo.Storage.Delete", Func: (*interopContext).storageDelete, Price: 100},
	{Name: "Neo.Storage.Get", Func: (*interopContext).storageGet, Price: 100},
	{Name: "Neo.Storage.GetContext", Func: (*interopContext).storageGetContext, Price: 1},
	{Name: "Neo.Storage.GetReadOnlyContext", Func: (*interopContext).storageGetReadOnlyContext, Price: 1},
	{Name: "Neo.Storage.Put", Func: (*interopContext).storagePut, Price: 0},
	{Name: "Neo.StorageContext.AsReadOnly", Func: (*interopContext).storageContextAsReadOnly, Price: 1},
	{Name: "Neo.Transaction.GetAttributes", Func: (*interopContext).txGetAttributes, Price: 1},
	{Name: "Neo.Transaction.GetHash", Func: (*interopContext).txGetHash, Price: 1},
	{Name: "Neo.Transaction.GetInputs", Func: (*interopContext).txGetInputs, Price: 1},
	{Name: "Neo.Transaction.GetOutputs", Func: (*interopContext).txGetOutputs, Price: 1},
	{Name: "Neo.Transaction.GetReferences", Func: (*interopContext).txGetReferences, Price: 200},
	{Name: "Neo.Transaction.GetType", Func: (*interopContext).txGetType, Price: 1},
	{Name: "Neo.Transaction.GetUnspentCoins", Func: (*interopContext).txGetUnspentCoins, Price: 200},
	{Name: "Neo.Transaction.GetWitnesses", Func: (*interopContext).txGetWitnesses, Price: 200},
	{Name: "Neo.Witness.GetVerificationScript", Func: (*interopContext).witnessGetVerificationScript, Price: 100},
	//		{Name: "Neo.Enumerator.Concat", Func: (*interopContext).enumeratorConcat, Price: 1},
	//		{Name: "Neo.Enumerator.Create", Func: (*interopContext).enumeratorCreate, Price: 1},
	//		{Name: "Neo.Enumerator.Next", Func: (*interopContext).enumeratorNext, Price: 1},
	//		{Name: "Neo.Enumerator.Value", Func: (*interopContext).enumeratorValue, Price: 1},
	//		{Name: "Neo.Iterator.Concat", Func: (*interopContext).iteratorConcat, Price: 1},
	//		{Name: "Neo.Iterator.Create", Func: (*interopContext).iteratorCreate, Price: 1},
	//		{Name: "Neo.Iterator.Key", Func: (*interopContext).iteratorKey, Price: 1},
	//		{Name: "Neo.Iterator.Keys", Func: (*interopContext).iteratorKeys, Price: 1},
	//		{Name: "Neo.Iterator.Values", Func: (*interopContext).iteratorValues, Price: 1},
	{Name: "Neo.Runtime.Deserialize", Func: (*interopContext).runtimeDeserialize, Price: 1},
	{Name: "Neo.Runtime.Serialize", Func: (*interopContext).runtimeSerialize, Price: 1},
	//		{Name: "Neo.Storage.Find",                Func: (*interopContext).storageFind, Price: 1},

	// Aliases.
	//		{Name: "Neo.Iterator.Next", Func: (*interopContext).enumeratorNext, Price: 1},
	//		{Name: "Neo.Iterator.Value", Func: (*interopContext).enumeratorValue, Price: 1},

	// Old compatibility APIs.
	{Name: "AntShares.Account.GetBalance", Func: (*interopContext).accountGetBalance, Price: 1},
	{Name: "AntShares.Account.GetScriptHash", Func: (*interopContext).accountGetScriptHash, Price: 1},
	{Name: "AntShares.Account.GetVotes", Func: (*interopContext).accountGetVotes, Price: 1},
	{Name: "AntShares.Asset.Create", Func: (*interopContext).assetCreate, Price: 0},
	{Name: "AntShares.Asset.GetAdmin", Func: (*interopContext).assetGetAdmin, Price: 1},
	{Name: "AntShares.Asset.GetAmount", Func: (*interopContext).assetGetAmount, Price: 1},
	{Name: "AntShares.Asset.GetAssetId", Func: (*interopContext).assetGetAssetID, Price: 1},
	{Name: "AntShares.Asset.GetAssetType", Func: (*interopContext).assetGetAssetType, Price: 1},
	{Name: "AntShares.Asset.GetAvailable", Func: (*interopContext).assetGetAvailable, Price: 1},
	{Name: "AntShares.Asset.GetIssuer", Func: (*interopContext).assetGetIssuer, Price: 1},
	{Name: "AntShares.Asset.GetOwner", Func: (*interopContext).assetGetOwner, Price: 1},
	{Name: "AntShares.Asset.GetPrecision", Func: (*interopContext).assetGetPrecision, Price: 1},
	{Name: "AntShares.Asset.Renew", Func: (*interopContext).assetRenew, Price: 0},
	{Name: "AntShares.Attribute.GetData", Func: (*interopContext).attrGetData, Price: 1},
	{Name: "AntShares.Attribute.GetUsage", Func: (*interopContext).attrGetUsage, Price: 1},
	{Name: "AntShares.Block.GetTransaction", Func: (*interopContext).blockGetTransaction, Price: 1},
	{Name: "AntShares.Block.GetTransactionCount", Func: (*interopContext).blockGetTransactionCount, Price: 1},
	{Name: "AntShares.Block.GetTransactions", Func: (*interopContext).blockGetTransactions, Price: 1},
	{Name: "AntShares.Blockchain.GetAccount", Func: (*interopContext).bcGetAccount, Price: 100},
	{Name: "AntShares.Blockchain.GetAsset", Func: (*interopContext).bcGetAsset, Price: 100},
	{Name: "AntShares.Blockchain.GetBlock", Func: (*interopContext).bcGetBlock, Price: 200},
	{Name: "AntShares.Blockchain.GetContract", Func: (*interopContext).bcGetContract, Price: 100},
	{Name: "AntShares.Blockchain.GetHeader", Func: (*interopContext).bcGetHeader, Price: 100},
	{Name: "AntShares.Blockchain.GetHeight", Func: (*interopContext).bcGetHeight, Price: 1},
	{Name: "AntShares.Blockchain.GetTransaction", Func: (*interopContext).bcGetTransaction, Price: 100},
	{Name: "AntShares.Blockchain.GetValidators", Func: (*interopContext).bcGetValidators, Price: 200},
	{Name: "AntShares.Contract.Create", Func: (*interopContext).contractCreate, Price: 0},
	{Name: "AntShares.Contract.Destroy", Func: (*interopContext).contractDestroy, Price: 1},
	{Name: "AntShares.Contract.GetScript", Func: (*interopContext).contractGetScript, Price: 1},
	{Name: "AntShares.Contract.GetStorageContext", Func: (*interopContext).contractGetStorageContext, Price: 1},
	{Name: "AntShares.Contract.Migrate", Func: (*interopContext).contractMigrate, Price: 0},
	{Name: "AntShares.Header.GetConsensusData", Func: (*interopContext).headerGetConsensusData, Price: 1},
	{Name: "AntShares.Header.GetHash", Func: (*interopContext).headerGetHash, Price: 1},
	{Name: "AntShares.Header.GetMerkleRoot", Func: (*interopContext).headerGetMerkleRoot, Price: 1},
	{Name: "AntShares.Header.GetNextConsensus", Func: (*interopContext).headerGetNextConsensus, Price: 1},
	{Name: "AntShares.Header.GetPrevHash", Func: (*interopContext).headerGetPrevHash, Price: 1},
	{Name: "AntShares.Header.GetTimestamp", Func: (*interopContext).headerGetTimestamp, Price: 1},
	{Name: "AntShares.Header.GetVersion", Func: (*interopContext).headerGetVersion, Price: 1},
	{Name: "AntShares.Input.GetHash", Func: (*interopContext).inputGetHash, Price: 1},
	{Name: "AntShares.Input.GetIndex", Func: (*interopContext).inputGetIndex, Price: 1},
	{Name: "AntShares.Output.GetAssetId", Func: (*interopContext).outputGetAssetID, Price: 1},
	{Name: "AntShares.Output.GetScriptHash", Func: (*interopContext).outputGetScriptHash, Price: 1},
	{Name: "AntShares.Output.GetValue", Func: (*interopContext).outputGetValue, Price: 1},
	{Name: "AntShares.Runtime.CheckWitness", Func: (*interopContext).runtimeCheckWitness, Price: 200},
	{Name: "AntShares.Runtime.Log", Func: (*interopContext).runtimeLog, Price: 1},
	{Name: "AntShares.Runtime.Notify", Func: (*interopContext).runtimeNotify, Price: 1},
	{Name: "AntShares.Storage.Delete", Func: (*interopContext).storageDelete, Price: 100},
	{Name: "AntShares.Storage.Get", Func: (*interopContext).storageGet, Price: 100},
	{Name: "AntShares.Storage.GetContext", Func: (*interopContext).storageGetContext, Price: 1},
	{Name: "AntShares.Storage.Put", Func: (*interopContext).storagePut, Price: 0},
	{Name: "AntShares.Transaction.GetAttributes", Func: (*interopContext).txGetAttributes, Price: 1},
	{Name: "AntShares.Transaction.GetHash", Func: (*interopContext).txGetHash, Price: 1},
	{Name: "AntShares.Transaction.GetInputs", Func: (*interopContext).txGetInputs, Price: 1},
	{Name: "AntShares.Transaction.GetOutputs", Func: (*interopContext).txGetOutputs, Price: 1},
	{Name: "AntShares.Transaction.GetReferences", Func: (*interopContext).txGetReferences, Price: 200},
	{Name: "AntShares.Transaction.GetType", Func: (*interopContext).txGetType, Price: 1},
}

// initIDinInteropsSlice initializes IDs from names in one given
// interopedFunction slice and then sorts it.
func initIDinInteropsSlice(iops []interopedFunction) {
	for i := range iops {
		iops[i].ID = vm.InteropNameToID([]byte(iops[i].Name))
	}
	sort.Slice(iops, func(i, j int) bool {
		return iops[i].ID < iops[j].ID
	})
}

// init initializes IDs in the global interop slices.
func init() {
	initIDinInteropsSlice(systemInterops)
	initIDinInteropsSlice(neoInterops)
}
