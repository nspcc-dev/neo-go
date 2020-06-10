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
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
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
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return slice[n].Func(ic, v)
				},
				Price:         slice[n].Price,
				RequiredFlags: slice[n].RequiredFlags,
			}
		}
		return nil
	}
}

// All lists are sorted, keep 'em this way, please.
var systemInterops = []interop.Function{
	{Name: "System.Blockchain.GetBlock", Func: bcGetBlock, Price: 250, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Blockchain.GetContract", Func: bcGetContract, Price: 100, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Blockchain.GetHeight", Func: bcGetHeight, Price: 1, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Blockchain.GetTransaction", Func: bcGetTransaction, Price: 100, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Blockchain.GetTransactionFromBlock", Func: bcGetTransactionFromBlock, Price: 100, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Blockchain.GetTransactionHeight", Func: bcGetTransactionHeight, Price: 100, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Contract.Call", Func: contractCall, Price: 1, RequiredFlags: smartcontract.AllowCall},
	{Name: "System.Contract.CallEx", Func: contractCallEx, Price: 1, RequiredFlags: smartcontract.AllowCall},
	{Name: "System.Contract.Create", Func: contractCreate, Price: 0, RequiredFlags: smartcontract.AllowModifyStates},
	{Name: "System.Contract.Destroy", Func: contractDestroy, Price: 1, RequiredFlags: smartcontract.AllowModifyStates},
	{Name: "System.Contract.Update", Func: contractUpdate, Price: 0, RequiredFlags: smartcontract.AllowModifyStates},
	{Name: "System.Enumerator.Concat", Func: enumerator.Concat, Price: 1},
	{Name: "System.Enumerator.Create", Func: enumerator.Create, Price: 1},
	{Name: "System.Enumerator.Next", Func: enumerator.Next, Price: 1},
	{Name: "System.Enumerator.Value", Func: enumerator.Value, Price: 1},
	{Name: "System.ExecutionEngine.GetCallingScriptHash", Func: engineGetCallingScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetEntryScriptHash", Func: engineGetEntryScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetExecutingScriptHash", Func: engineGetExecutingScriptHash, Price: 1},
	{Name: "System.ExecutionEngine.GetScriptContainer", Func: engineGetScriptContainer, Price: 1},
	{Name: "System.Iterator.Concat", Func: iterator.Concat, Price: 1},
	{Name: "System.Iterator.Create", Func: iterator.Create, Price: 1},
	{Name: "System.Iterator.Key", Func: iterator.Key, Price: 1},
	{Name: "System.Iterator.Keys", Func: iterator.Keys, Price: 1},
	{Name: "System.Iterator.Values", Func: iterator.Values, Price: 1},
	{Name: "System.Runtime.CheckWitness", Func: runtime.CheckWitness, Price: 200, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Runtime.Deserialize", Func: runtimeDeserialize, Price: 1},
	{Name: "System.Runtime.GetTime", Func: runtimeGetTime, Price: 1, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Runtime.GetTrigger", Func: runtimeGetTrigger, Price: 1},
	{Name: "System.Runtime.Log", Func: runtimeLog, Price: 1, RequiredFlags: smartcontract.AllowNotify},
	{Name: "System.Runtime.Notify", Func: runtimeNotify, Price: 1, RequiredFlags: smartcontract.AllowNotify},
	{Name: "System.Runtime.Platform", Func: runtimePlatform, Price: 1},
	{Name: "System.Runtime.Serialize", Func: runtimeSerialize, Price: 1},
	{Name: "System.Storage.Delete", Func: storageDelete, Price: 100, RequiredFlags: smartcontract.AllowModifyStates},
	{Name: "System.Storage.Find", Func: storageFind, Price: 1, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Storage.Get", Func: storageGet, Price: 100, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Storage.GetContext", Func: storageGetContext, Price: 1, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Storage.GetReadOnlyContext", Func: storageGetReadOnlyContext, Price: 1, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Storage.Put", Func: storagePut, Price: 0, RequiredFlags: smartcontract.AllowModifyStates}, // These don't have static price in C# code.
	{Name: "System.Storage.PutEx", Func: storagePutEx, Price: 0, RequiredFlags: smartcontract.AllowModifyStates},
	{Name: "System.Storage.AsReadOnly", Func: storageContextAsReadOnly, Price: 1, RequiredFlags: smartcontract.AllowStates},
}

var neoInterops = []interop.Function{
	{Name: "Neo.Crypto.ECDsaVerify", Func: crypto.ECDSAVerify, Price: 1},
	{Name: "Neo.Crypto.ECDsaCheckMultiSig", Func: crypto.ECDSACheckMultisig, Price: 1},
	{Name: "Neo.Crypto.SHA256", Func: crypto.Sha256, Price: 1},
	{Name: "Neo.Native.Deploy", Func: native.Deploy, Price: 1, RequiredFlags: smartcontract.AllowModifyStates},
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
