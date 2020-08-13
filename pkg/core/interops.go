package core

/*
  Interops are designed to run under VM's execute() panic protection, so it's OK
  for them to do things like
          smth := v.Estack().Pop().Bytes()
  even though technically Pop() can return a nil pointer.
*/

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/callback"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/enumerator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/json"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// SpawnVM returns a VM with script getter and interop functions set
// up for current blockchain.
func SpawnVM(ic *interop.Context) *vm.VM {
	vm := ic.SpawnVM()
	ic.Functions = [][]interop.Function{systemInterops, neoInterops}
	return vm
}

// All lists are sorted, keep 'em this way, please.
var systemInterops = []interop.Function{
	{Name: "System.Binary.Base64Decode", Func: runtimeDecode, Price: 100000, ParamCount: 1},
	{Name: "System.Binary.Base64Encode", Func: runtimeEncode, Price: 100000, ParamCount: 1},
	{Name: "System.Binary.Deserialize", Func: runtimeDeserialize, Price: 500000, ParamCount: 1},
	{Name: "System.Binary.Serialize", Func: runtimeSerialize, Price: 100000, ParamCount: 1},
	{Name: "System.Blockchain.GetBlock", Func: bcGetBlock, Price: 2500000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: "System.Blockchain.GetContract", Func: bcGetContract, Price: 1000000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: "System.Blockchain.GetHeight", Func: bcGetHeight, Price: 400,
		RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Blockchain.GetTransaction", Func: bcGetTransaction, Price: 1000000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: "System.Blockchain.GetTransactionFromBlock", Func: bcGetTransactionFromBlock, Price: 1000000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 2},
	{Name: "System.Blockchain.GetTransactionHeight", Func: bcGetTransactionHeight, Price: 1000000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: "System.Callback.Create", Func: callback.Create, Price: 400, ParamCount: 3, DisallowCallback: true},
	{Name: "System.Callback.CreateFromMethod", Func: callback.CreateFromMethod, Price: 1000000, ParamCount: 2, DisallowCallback: true},
	{Name: "System.Callback.CreateFromSyscall", Func: callback.CreateFromSyscall, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Callback.Invoke", Func: callback.Invoke, Price: 1000000, ParamCount: 2, DisallowCallback: true},
	{Name: "System.Contract.Call", Func: contractCall, Price: 1000000,
		RequiredFlags: smartcontract.AllowCall, ParamCount: 3, DisallowCallback: true},
	{Name: "System.Contract.CallEx", Func: contractCallEx, Price: 1000000,
		RequiredFlags: smartcontract.AllowCall, ParamCount: 4, DisallowCallback: true},
	{Name: "System.Contract.Create", Func: contractCreate, Price: 0,
		RequiredFlags: smartcontract.AllowModifyStates, ParamCount: 2, DisallowCallback: true},
	{Name: "System.Contract.CreateStandardAccount", Func: contractCreateStandardAccount, Price: 10000, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Contract.Destroy", Func: contractDestroy, Price: 1000000, RequiredFlags: smartcontract.AllowModifyStates, DisallowCallback: true},
	{Name: "System.Contract.IsStandard", Func: contractIsStandard, Price: 30000, ParamCount: 1},
	{Name: "System.Contract.GetCallFlags", Func: contractGetCallFlags, Price: 30000, DisallowCallback: true},
	{Name: "System.Contract.Update", Func: contractUpdate, Price: 0,
		RequiredFlags: smartcontract.AllowModifyStates, ParamCount: 2, DisallowCallback: true},
	{Name: "System.Enumerator.Concat", Func: enumerator.Concat, Price: 400, ParamCount: 2, DisallowCallback: true},
	{Name: "System.Enumerator.Create", Func: enumerator.Create, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Enumerator.Next", Func: enumerator.Next, Price: 1000000, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Enumerator.Value", Func: enumerator.Value, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Iterator.Concat", Func: iterator.Concat, Price: 400, ParamCount: 2, DisallowCallback: true},
	{Name: "System.Iterator.Create", Func: iterator.Create, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Iterator.Key", Func: iterator.Key, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Iterator.Keys", Func: iterator.Keys, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Iterator.Values", Func: iterator.Values, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: "System.Json.Deserialize", Func: json.Deserialize, Price: 500000, ParamCount: 1},
	{Name: "System.Json.Serialize", Func: json.Serialize, Price: 100000, ParamCount: 1},
	{Name: "System.Runtime.CheckWitness", Func: runtime.CheckWitness, Price: 30000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: "System.Runtime.GasLeft", Func: runtime.GasLeft, Price: 400},
	{Name: "System.Runtime.GetCallingScriptHash", Func: engineGetCallingScriptHash, Price: 400},
	{Name: "System.Runtime.GetEntryScriptHash", Func: engineGetEntryScriptHash, Price: 400},
	{Name: "System.Runtime.GetExecutingScriptHash", Func: engineGetExecutingScriptHash, Price: 400},
	{Name: "System.Runtime.GetInvocationCounter", Func: runtime.GetInvocationCounter, Price: 400},
	{Name: "System.Runtime.GetNotifications", Func: runtime.GetNotifications, Price: 10000, ParamCount: 1},
	{Name: "System.Runtime.GetScriptContainer", Func: engineGetScriptContainer, Price: 250},
	{Name: "System.Runtime.GetTime", Func: runtimeGetTime, Price: 250, RequiredFlags: smartcontract.AllowStates},
	{Name: "System.Runtime.GetTrigger", Func: runtimeGetTrigger, Price: 250},
	{Name: "System.Runtime.Log", Func: runtimeLog, Price: 1000000, RequiredFlags: smartcontract.AllowNotify,
		ParamCount: 1, DisallowCallback: true},
	{Name: "System.Runtime.Notify", Func: runtimeNotify, Price: 1000000, RequiredFlags: smartcontract.AllowNotify,
		ParamCount: 2, DisallowCallback: true},
	{Name: "System.Runtime.Platform", Func: runtimePlatform, Price: 250},
	{Name: "System.Storage.Delete", Func: storageDelete, Price: StoragePrice,
		RequiredFlags: smartcontract.AllowModifyStates, ParamCount: 2, DisallowCallback: true},
	{Name: "System.Storage.Find", Func: storageFind, Price: 1000000, RequiredFlags: smartcontract.AllowStates,
		ParamCount: 2, DisallowCallback: true},
	{Name: "System.Storage.Get", Func: storageGet, Price: 1000000, RequiredFlags: smartcontract.AllowStates,
		ParamCount: 2, DisallowCallback: true},
	{Name: "System.Storage.GetContext", Func: storageGetContext, Price: 400,
		RequiredFlags: smartcontract.AllowStates, DisallowCallback: true},
	{Name: "System.Storage.GetReadOnlyContext", Func: storageGetReadOnlyContext, Price: 400,
		RequiredFlags: smartcontract.AllowStates, DisallowCallback: true},
	{Name: "System.Storage.Put", Func: storagePut, Price: 0, RequiredFlags: smartcontract.AllowModifyStates,
		ParamCount: 3, DisallowCallback: true}, // These don't have static price in C# code.
	{Name: "System.Storage.PutEx", Func: storagePutEx, Price: 0, RequiredFlags: smartcontract.AllowModifyStates,
		ParamCount: 4, DisallowCallback: true},
	{Name: "System.Storage.AsReadOnly", Func: storageContextAsReadOnly, Price: 400,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1, DisallowCallback: true},
}

var neoInterops = []interop.Function{
	{Name: "Neo.Crypto.VerifyWithECDsaSecp256r1", Func: crypto.ECDSASecp256r1Verify,
		Price: crypto.ECDSAVerifyPrice, ParamCount: 3},
	{Name: "Neo.Crypto.VerifyWithECDsaSecp256k1", Func: crypto.ECDSASecp256k1Verify,
		Price: crypto.ECDSAVerifyPrice, ParamCount: 3},
	{Name: "Neo.Crypto.CheckMultisigWithECDsaSecp256r1", Func: crypto.ECDSASecp256r1CheckMultisig, Price: 0, ParamCount: 3},
	{Name: "Neo.Crypto.CheckMultisigWithECDsaSecp256k1", Func: crypto.ECDSASecp256k1CheckMultisig, Price: 0, ParamCount: 3},
	{Name: "Neo.Crypto.SHA256", Func: crypto.Sha256, Price: 1000000, ParamCount: 1},
	{Name: "Neo.Crypto.RIPEMD160", Func: crypto.RipeMD160, Price: 1000000, ParamCount: 1},
	{Name: "Neo.Native.Call", Func: native.Call, Price: 0, ParamCount: 1, DisallowCallback: true},
	{Name: "Neo.Native.Deploy", Func: native.Deploy, Price: 0, RequiredFlags: smartcontract.AllowModifyStates, DisallowCallback: true},
}

// initIDinInteropsSlice initializes IDs from names in one given
// Function slice and then sorts it.
func initIDinInteropsSlice(iops []interop.Function) {
	for i := range iops {
		iops[i].ID = interopnames.ToID([]byte(iops[i].Name))
	}
	interop.Sort(iops)
}

// init initializes IDs in the global interop slices.
func init() {
	initIDinInteropsSlice(systemInterops)
	initIDinInteropsSlice(neoInterops)
}
