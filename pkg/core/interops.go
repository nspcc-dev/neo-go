package core

/*
  Interops are designed to run under VM's execute() panic protection, so it's OK
  for them to do things like
          smth := v.Estack().Pop().Bytes()
  even though technically Pop() can return a nil pointer.
*/

import (
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// SpawnVM returns a VM with script getter and interop functions set
// up for current blockchain.
func SpawnVM(ic *interop.Context) *vm.VM {
	vm := ic.SpawnVM()
	ic.Functions = systemInterops
	return vm
}

// All lists are sorted, keep 'em this way, please.
var systemInterops = []interop.Function{
	{Name: interopnames.SystemContractCall, Func: contract.Call, Price: 1 << 15,
		RequiredFlags: callflag.ReadStates | callflag.AllowCall, ParamCount: 4},
	{Name: interopnames.SystemContractCallNative, Func: native.Call, Price: 0, ParamCount: 1},
	{Name: interopnames.SystemContractCreateMultisigAccount, Func: contract.CreateMultisigAccount, Price: 0, ParamCount: 2},
	{Name: interopnames.SystemContractCreateStandardAccount, Func: contract.CreateStandardAccount, Price: 0, ParamCount: 1},
	{Name: interopnames.SystemContractGetCallFlags, Func: contract.GetCallFlags, Price: 1 << 10},
	{Name: interopnames.SystemContractNativeOnPersist, Func: native.OnPersist, Price: 0, RequiredFlags: callflag.States},
	{Name: interopnames.SystemContractNativePostPersist, Func: native.PostPersist, Price: 0, RequiredFlags: callflag.States},
	{Name: interopnames.SystemCryptoCheckMultisig, Func: crypto.ECDSASecp256r1CheckMultisig, Price: 0, ParamCount: 2},
	{Name: interopnames.SystemCryptoCheckSig, Func: crypto.ECDSASecp256r1CheckSig, Price: fee.ECDSAVerifyPrice, ParamCount: 2},
	{Name: interopnames.SystemIteratorNext, Func: iterator.Next, Price: 1 << 15, ParamCount: 1},
	{Name: interopnames.SystemIteratorValue, Func: iterator.Value, Price: 1 << 4, ParamCount: 1},
	{Name: interopnames.SystemRuntimeBurnGas, Func: runtime.BurnGas, Price: 1 << 4, ParamCount: 1},
	{Name: interopnames.SystemRuntimeCheckWitness, Func: runtime.CheckWitness, Price: 1 << 10,
		RequiredFlags: callflag.NoneFlag, ParamCount: 1},
	{Name: interopnames.SystemRuntimeGasLeft, Func: runtime.GasLeft, Price: 1 << 4},
	{Name: interopnames.SystemRuntimeGetAddressVersion, Func: runtime.GetAddressVersion, Price: 1 << 3},
	{Name: interopnames.SystemRuntimeGetCallingScriptHash, Func: runtime.GetCallingScriptHash, Price: 1 << 4},
	{Name: interopnames.SystemRuntimeGetEntryScriptHash, Func: runtime.GetEntryScriptHash, Price: 1 << 4},
	{Name: interopnames.SystemRuntimeGetExecutingScriptHash, Func: runtime.GetExecutingScriptHash, Price: 1 << 4},
	{Name: interopnames.SystemRuntimeGetInvocationCounter, Func: runtime.GetInvocationCounter, Price: 1 << 4},
	{Name: interopnames.SystemRuntimeGetNetwork, Func: runtime.GetNetwork, Price: 1 << 3},
	{Name: interopnames.SystemRuntimeGetNotifications, Func: runtime.GetNotifications, Price: 1 << 12, ParamCount: 1},
	{Name: interopnames.SystemRuntimeGetRandom, Func: runtime.GetRandom, Price: 0},
	{Name: interopnames.SystemRuntimeGetScriptContainer, Func: runtime.GetScriptContainer, Price: 1 << 3},
	{Name: interopnames.SystemRuntimeGetTime, Func: runtime.GetTime, Price: 1 << 3, RequiredFlags: callflag.ReadStates},
	{Name: interopnames.SystemRuntimeGetTrigger, Func: runtime.GetTrigger, Price: 1 << 3},
	{Name: interopnames.SystemRuntimeLoadScript, Func: runtime.LoadScript, Price: 1 << 15, RequiredFlags: callflag.AllowCall,
		ParamCount: 3},
	{Name: interopnames.SystemRuntimeLog, Func: runtime.Log, Price: 1 << 15, RequiredFlags: callflag.AllowNotify,
		ParamCount: 1},
	{Name: interopnames.SystemRuntimeNotify, Func: runtime.Notify, Price: 1 << 15, RequiredFlags: callflag.AllowNotify,
		ParamCount: 2},
	{Name: interopnames.SystemRuntimePlatform, Func: runtime.Platform, Price: 1 << 3},
	{Name: interopnames.SystemStorageDelete, Func: storage.Delete, Price: 1 << 15,
		RequiredFlags: callflag.WriteStates, ParamCount: 2},
	{Name: interopnames.SystemStorageFind, Func: storage.Find, Price: 1 << 15, RequiredFlags: callflag.ReadStates,
		ParamCount: 3},
	{Name: interopnames.SystemStorageGet, Func: storage.Get, Price: 1 << 15, RequiredFlags: callflag.ReadStates,
		ParamCount: 2},
	{Name: interopnames.SystemStorageGetContext, Func: storage.GetContext, Price: 1 << 4,
		RequiredFlags: callflag.ReadStates},
	{Name: interopnames.SystemStorageGetReadOnlyContext, Func: storage.GetReadOnlyContext, Price: 1 << 4,
		RequiredFlags: callflag.ReadStates},
	{Name: interopnames.SystemStoragePut, Func: storage.Put, Price: 1 << 15, RequiredFlags: callflag.WriteStates,
		ParamCount: 3},
	{Name: interopnames.SystemStorageAsReadOnly, Func: storage.ContextAsReadOnly, Price: 1 << 4,
		RequiredFlags: callflag.ReadStates, ParamCount: 1},
}

// init initializes IDs in the global interop slices.
func init() {
	for i := range systemInterops {
		systemInterops[i].ID = interopnames.ToID([]byte(systemInterops[i].Name))
	}
	interop.Sort(systemInterops)
}
