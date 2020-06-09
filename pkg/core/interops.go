package core

/*
  Interops are designed to run under VM's execute() panic protection, so it's OK
  for them to do things like
          smth := v.Estack().Pop().Bytes()
  even though technically Pop() can return a nil pointer.
*/

import (
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/enumerator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// SpawnVM returns a VM with script getter and interop functions set
// up for current blockchain.
func SpawnVM(ic *interop.Context) *vm.VM {
	vm := vm.New()
	vm.RegisterInteropGetter(getSystemInterop(ic))
	vm.RegisterInteropGetter(getNeoInterop(ic))
	if ic.Chain != nil {
		vm.RegisterInteropGetter(ic.Chain.(*Blockchain).contracts.GetNativeInterop(ic))
	}
	return vm
}

// getSystemInterop returns matching interop function from the System namespace
// for a given id in the current context.
func getSystemInterop(ic *interop.Context) vm.InteropGetterFunc {
	return getInteropFromSlice(ic, systemInterops)
}

// getNeoInterop returns matching interop function from the Neo and AntShares
// namespaces for a given id in the current context.
func getNeoInterop(ic *interop.Context) vm.InteropGetterFunc {
	return getInteropFromSlice(ic, neoInterops)
}

// getInteropFromSlice returns matching interop function from the given slice of
// interop functions in the current context.
func getInteropFromSlice(ic *interop.Context, slice []interop.Function) func(uint32) *vm.InteropFuncPrice {
	return func(id uint32) *vm.InteropFuncPrice {
		n := sort.Search(len(slice), func(i int) bool {
			return slice[i].ID >= id
		})
		if n < len(slice) && slice[n].ID == id {
			return &vm.InteropFuncPrice{Func: func(v *vm.VM) error {
				return slice[n].Func(ic, v)
			}, Price: slice[n].Price}
		}
		return nil
	}
}

// All lists are sorted, keep 'em this way, please.
var systemInterops = []interop.Function{
	{Name: "System.Block.GetTransaction", Func: blockGetTransaction, Price: 1},
	{Name: "System.Block.GetTransactionCount", Func: blockGetTransactionCount, Price: 1},
	{Name: "System.Block.GetTransactions", Func: blockGetTransactions, Price: 1},
	{Name: "System.Blockchain.GetBlock", Func: bcGetBlock, Price: 200},
	{Name: "System.Blockchain.GetContract", Func: bcGetContract, Price: 100},
	{Name: "System.Blockchain.GetHeader", Func: bcGetHeader, Price: 100},
	{Name: "System.Blockchain.GetHeight", Func: bcGetHeight, Price: 1},
	{Name: "System.Blockchain.GetTransaction", Func: bcGetTransaction, Price: 200},
	{Name: "System.Blockchain.GetTransactionHeight", Func: bcGetTransactionHeight, Price: 100},
	{Name: "System.Contract.Call", Func: contractCall, Price: 1},
	{Name: "System.Contract.CallEx", Func: contractCallEx, Price: 1},
	{Name: "System.Contract.Destroy", Func: contractDestroy, Price: 1},
	{Name: "System.Contract.GetStorageContext", Func: contractGetStorageContext, Price: 1},
	{Name: "System.ExecutionEngine.GetCallingScriptHash", Func: engineGetCallingScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetEntryScriptHash", Func: engineGetEntryScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetExecutingScriptHash", Func: engineGetExecutingScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetScriptContainer", Func: engineGetScriptContainer, Price: 1},
	{Name: "System.Header.GetHash", Func: headerGetHash, Price: 1},
	{Name: "System.Header.GetIndex", Func: headerGetIndex, Price: 1},
	{Name: "System.Header.GetPrevHash", Func: headerGetPrevHash, Price: 1},
	{Name: "System.Header.GetTimestamp", Func: headerGetTimestamp, Price: 1},
	{Name: "System.Runtime.CheckWitness", Func: runtime.CheckWitness, Price: 200},
	{Name: "System.Runtime.Deserialize", Func: runtimeDeserialize, Price: 1},
	{Name: "System.Runtime.GetTime", Func: runtimeGetTime, Price: 1},
	{Name: "System.Runtime.GetTrigger", Func: runtimeGetTrigger, Price: 1},
	{Name: "System.Runtime.Log", Func: runtimeLog, Price: 1},
	{Name: "System.Runtime.Notify", Func: runtimeNotify, Price: 1},
	{Name: "System.Runtime.Platform", Func: runtimePlatform, Price: 1},
	{Name: "System.Runtime.Serialize", Func: runtimeSerialize, Price: 1},
	{Name: "System.Storage.Delete", Func: storageDelete, Price: 100},
	{Name: "System.Storage.Get", Func: storageGet, Price: 100},
	{Name: "System.Storage.GetContext", Func: storageGetContext, Price: 1},
	{Name: "System.Storage.GetReadOnlyContext", Func: storageGetReadOnlyContext, Price: 1},
	{Name: "System.Storage.Put", Func: storagePut, Price: 0}, // These don't have static price in C# code.
	{Name: "System.Storage.PutEx", Func: storagePutEx, Price: 0},
	{Name: "System.StorageContext.AsReadOnly", Func: storageContextAsReadOnly, Price: 1},
	{Name: "System.Transaction.GetHash", Func: txGetHash, Price: 1},
}

var neoInterops = []interop.Function{
	{Name: "Neo.Account.GetBalance", Func: accountGetBalance, Price: 1},
	{Name: "Neo.Account.GetScriptHash", Func: accountGetScriptHash, Price: 1},
	{Name: "Neo.Account.IsStandard", Func: accountIsStandard, Price: 100},
	{Name: "Neo.Attribute.GetData", Func: attrGetData, Price: 1},
	{Name: "Neo.Attribute.GetUsage", Func: attrGetUsage, Price: 1},
	{Name: "Neo.Block.GetTransaction", Func: blockGetTransaction, Price: 1},
	{Name: "Neo.Block.GetTransactionCount", Func: blockGetTransactionCount, Price: 1},
	{Name: "Neo.Block.GetTransactions", Func: blockGetTransactions, Price: 1},
	{Name: "Neo.Blockchain.GetAccount", Func: bcGetAccount, Price: 100},
	{Name: "Neo.Blockchain.GetBlock", Func: bcGetBlock, Price: 200},
	{Name: "Neo.Blockchain.GetContract", Func: bcGetContract, Price: 100},
	{Name: "Neo.Blockchain.GetHeader", Func: bcGetHeader, Price: 100},
	{Name: "Neo.Blockchain.GetHeight", Func: bcGetHeight, Price: 1},
	{Name: "Neo.Blockchain.GetTransaction", Func: bcGetTransaction, Price: 100},
	{Name: "Neo.Blockchain.GetTransactionHeight", Func: bcGetTransactionHeight, Price: 100},
	{Name: "Neo.Contract.Create", Func: contractCreate, Price: 0},
	{Name: "Neo.Contract.Destroy", Func: contractDestroy, Price: 1},
	{Name: "Neo.Contract.GetScript", Func: contractGetScript, Price: 1},
	{Name: "Neo.Contract.GetStorageContext", Func: contractGetStorageContext, Price: 1},
	{Name: "Neo.Contract.IsPayable", Func: contractIsPayable, Price: 1},
	{Name: "Neo.Contract.Migrate", Func: contractMigrate, Price: 0},
	{Name: "Neo.Crypto.ECDsaVerify", Func: crypto.ECDSAVerify, Price: 1},
	{Name: "Neo.Crypto.ECDsaCheckMultiSig", Func: crypto.ECDSACheckMultisig, Price: 1},
	{Name: "Neo.Crypto.SHA256", Func: crypto.Sha256, Price: 1},
	{Name: "Neo.Enumerator.Concat", Func: enumerator.Concat, Price: 1},
	{Name: "Neo.Enumerator.Create", Func: enumerator.Create, Price: 1},
	{Name: "Neo.Enumerator.Next", Func: enumerator.Next, Price: 1},
	{Name: "Neo.Enumerator.Value", Func: enumerator.Value, Price: 1},
	{Name: "Neo.Header.GetHash", Func: headerGetHash, Price: 1},
	{Name: "Neo.Header.GetIndex", Func: headerGetIndex, Price: 1},
	{Name: "Neo.Header.GetMerkleRoot", Func: headerGetMerkleRoot, Price: 1},
	{Name: "Neo.Header.GetNextConsensus", Func: headerGetNextConsensus, Price: 1},
	{Name: "Neo.Header.GetPrevHash", Func: headerGetPrevHash, Price: 1},
	{Name: "Neo.Header.GetTimestamp", Func: headerGetTimestamp, Price: 1},
	{Name: "Neo.Header.GetVersion", Func: headerGetVersion, Price: 1},
	{Name: "Neo.Iterator.Concat", Func: iterator.Concat, Price: 1},
	{Name: "Neo.Iterator.Create", Func: iterator.Create, Price: 1},
	{Name: "Neo.Iterator.Key", Func: iterator.Key, Price: 1},
	{Name: "Neo.Iterator.Keys", Func: iterator.Keys, Price: 1},
	{Name: "Neo.Iterator.Values", Func: iterator.Values, Price: 1},
	{Name: "Neo.Native.Deploy", Func: native.Deploy, Price: 1},
	{Name: "Neo.Runtime.CheckWitness", Func: runtime.CheckWitness, Price: 200},
	{Name: "Neo.Runtime.Deserialize", Func: runtimeDeserialize, Price: 1},
	{Name: "Neo.Runtime.GetTime", Func: runtimeGetTime, Price: 1},
	{Name: "Neo.Runtime.GetTrigger", Func: runtimeGetTrigger, Price: 1},
	{Name: "Neo.Runtime.Log", Func: runtimeLog, Price: 1},
	{Name: "Neo.Runtime.Notify", Func: runtimeNotify, Price: 1},
	{Name: "Neo.Runtime.Serialize", Func: runtimeSerialize, Price: 1},
	{Name: "Neo.Storage.Delete", Func: storageDelete, Price: 100},
	{Name: "Neo.Storage.Find", Func: storageFind, Price: 1},
	{Name: "Neo.Storage.Get", Func: storageGet, Price: 100},
	{Name: "Neo.Storage.GetContext", Func: storageGetContext, Price: 1},
	{Name: "Neo.Storage.GetReadOnlyContext", Func: storageGetReadOnlyContext, Price: 1},
	{Name: "Neo.Storage.Put", Func: storagePut, Price: 0},
	{Name: "Neo.StorageContext.AsReadOnly", Func: storageContextAsReadOnly, Price: 1},
	{Name: "Neo.Transaction.GetAttributes", Func: txGetAttributes, Price: 1},
	{Name: "Neo.Transaction.GetHash", Func: txGetHash, Price: 1},
	{Name: "Neo.Transaction.GetWitnesses", Func: txGetWitnesses, Price: 200},
	{Name: "Neo.Witness.GetVerificationScript", Func: witnessGetVerificationScript, Price: 100},

	// Aliases.
	{Name: "Neo.Iterator.Next", Func: enumerator.Next, Price: 1},
	{Name: "Neo.Iterator.Value", Func: enumerator.Value, Price: 1},
}

// initIDinInteropsSlice initializes IDs from names in one given
// Function slice and then sorts it.
func initIDinInteropsSlice(iops []interop.Function) {
	for i := range iops {
		iops[i].ID = emit.InteropNameToID([]byte(iops[i].Name))
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
