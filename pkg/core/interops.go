package core

/*
  Interops are designed to run under VM's execute() panic protection, so it's OK
  for them to do things like
          smth := v.Estack().Pop().Bytes()
  even though technically Pop() can return a nil pointer.
*/

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/binary"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/callback"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
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
	{Name: interopnames.SystemBinaryAtoi, Func: binary.Atoi, Price: 100000, ParamCount: 2},
	{Name: interopnames.SystemBinaryBase58Decode, Func: binary.DecodeBase58, Price: 100000, ParamCount: 1},
	{Name: interopnames.SystemBinaryBase58Encode, Func: binary.EncodeBase58, Price: 100000, ParamCount: 1},
	{Name: interopnames.SystemBinaryBase64Decode, Func: binary.DecodeBase64, Price: 100000, ParamCount: 1},
	{Name: interopnames.SystemBinaryBase64Encode, Func: binary.EncodeBase64, Price: 100000, ParamCount: 1},
	{Name: interopnames.SystemBinaryDeserialize, Func: binary.Deserialize, Price: 500000, ParamCount: 1},
	{Name: interopnames.SystemBinaryItoa, Func: binary.Itoa, Price: 100000, ParamCount: 2},
	{Name: interopnames.SystemBinarySerialize, Func: binary.Serialize, Price: 100000, ParamCount: 1},
	{Name: interopnames.SystemBlockchainGetBlock, Func: bcGetBlock, Price: 2500000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: interopnames.SystemBlockchainGetContract, Func: bcGetContract, Price: 1000000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: interopnames.SystemBlockchainGetHeight, Func: bcGetHeight, Price: 400,
		RequiredFlags: smartcontract.AllowStates},
	{Name: interopnames.SystemBlockchainGetTransaction, Func: bcGetTransaction, Price: 1000000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: interopnames.SystemBlockchainGetTransactionFromBlock, Func: bcGetTransactionFromBlock, Price: 1000000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 2},
	{Name: interopnames.SystemBlockchainGetTransactionHeight, Func: bcGetTransactionHeight, Price: 1000000,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: interopnames.SystemCallbackCreate, Func: callback.Create, Price: 400, ParamCount: 3, DisallowCallback: true},
	{Name: interopnames.SystemCallbackCreateFromMethod, Func: callback.CreateFromMethod, Price: 1000000, ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemCallbackCreateFromSyscall, Func: callback.CreateFromSyscall, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemCallbackInvoke, Func: callback.Invoke, Price: 1000000, ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemContractCall, Func: contract.Call, Price: 1000000,
		RequiredFlags: smartcontract.AllowCall, ParamCount: 3, DisallowCallback: true},
	{Name: interopnames.SystemContractCallEx, Func: contract.CallEx, Price: 1000000,
		RequiredFlags: smartcontract.AllowCall, ParamCount: 4, DisallowCallback: true},
	{Name: interopnames.SystemContractCreate, Func: contractCreate, Price: 0,
		RequiredFlags: smartcontract.AllowModifyStates, ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemContractCreateStandardAccount, Func: contractCreateStandardAccount, Price: 10000, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemContractDestroy, Func: contractDestroy, Price: 1000000, RequiredFlags: smartcontract.AllowModifyStates, DisallowCallback: true},
	{Name: interopnames.SystemContractIsStandard, Func: contractIsStandard, Price: 30000, RequiredFlags: smartcontract.AllowStates, ParamCount: 1},
	{Name: interopnames.SystemContractGetCallFlags, Func: contractGetCallFlags, Price: 30000, DisallowCallback: true},
	{Name: interopnames.SystemContractUpdate, Func: contractUpdate, Price: 0,
		RequiredFlags: smartcontract.AllowModifyStates, ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemEnumeratorConcat, Func: enumerator.Concat, Price: 400, ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemEnumeratorCreate, Func: enumerator.Create, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemEnumeratorNext, Func: enumerator.Next, Price: 1000000, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemEnumeratorValue, Func: enumerator.Value, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemIteratorConcat, Func: iterator.Concat, Price: 400, ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemIteratorCreate, Func: iterator.Create, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemIteratorKey, Func: iterator.Key, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemIteratorKeys, Func: iterator.Keys, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemIteratorValues, Func: iterator.Values, Price: 400, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemJSONDeserialize, Func: json.Deserialize, Price: 500000, ParamCount: 1},
	{Name: interopnames.SystemJSONSerialize, Func: json.Serialize, Price: 100000, ParamCount: 1},
	{Name: interopnames.SystemRuntimeCheckWitness, Func: runtime.CheckWitness, Price: 30000,
		RequiredFlags: smartcontract.NoneFlag, ParamCount: 1},
	{Name: interopnames.SystemRuntimeGasLeft, Func: runtime.GasLeft, Price: 400},
	{Name: interopnames.SystemRuntimeGetCallingScriptHash, Func: runtime.GetCallingScriptHash, Price: 400},
	{Name: interopnames.SystemRuntimeGetEntryScriptHash, Func: runtime.GetEntryScriptHash, Price: 400},
	{Name: interopnames.SystemRuntimeGetExecutingScriptHash, Func: runtime.GetExecutingScriptHash, Price: 400},
	{Name: interopnames.SystemRuntimeGetInvocationCounter, Func: runtime.GetInvocationCounter, Price: 400},
	{Name: interopnames.SystemRuntimeGetNotifications, Func: runtime.GetNotifications, Price: 10000, ParamCount: 1},
	{Name: interopnames.SystemRuntimeGetScriptContainer, Func: engineGetScriptContainer, Price: 250},
	{Name: interopnames.SystemRuntimeGetTime, Func: runtime.GetTime, Price: 250, RequiredFlags: smartcontract.AllowStates},
	{Name: interopnames.SystemRuntimeGetTrigger, Func: runtime.GetTrigger, Price: 250},
	{Name: interopnames.SystemRuntimeLog, Func: runtime.Log, Price: 1000000, RequiredFlags: smartcontract.AllowNotify,
		ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.SystemRuntimeNotify, Func: runtime.Notify, Price: 1000000, RequiredFlags: smartcontract.AllowNotify,
		ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemRuntimePlatform, Func: runtime.Platform, Price: 250},
	{Name: interopnames.SystemStorageDelete, Func: storageDelete, Price: StoragePrice,
		RequiredFlags: smartcontract.AllowModifyStates, ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemStorageFind, Func: storageFind, Price: 1000000, RequiredFlags: smartcontract.AllowStates,
		ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemStorageGet, Func: storageGet, Price: 1000000, RequiredFlags: smartcontract.AllowStates,
		ParamCount: 2, DisallowCallback: true},
	{Name: interopnames.SystemStorageGetContext, Func: storageGetContext, Price: 400,
		RequiredFlags: smartcontract.AllowStates, DisallowCallback: true},
	{Name: interopnames.SystemStorageGetReadOnlyContext, Func: storageGetReadOnlyContext, Price: 400,
		RequiredFlags: smartcontract.AllowStates, DisallowCallback: true},
	{Name: interopnames.SystemStoragePut, Func: storagePut, Price: 0, RequiredFlags: smartcontract.AllowModifyStates,
		ParamCount: 3, DisallowCallback: true}, // These don't have static price in C# code.
	{Name: interopnames.SystemStoragePutEx, Func: storagePutEx, Price: 0, RequiredFlags: smartcontract.AllowModifyStates,
		ParamCount: 4, DisallowCallback: true},
	{Name: interopnames.SystemStorageAsReadOnly, Func: storageContextAsReadOnly, Price: 400,
		RequiredFlags: smartcontract.AllowStates, ParamCount: 1, DisallowCallback: true},
}

var neoInterops = []interop.Function{
	{Name: interopnames.NeoCryptoVerifyWithECDsaSecp256r1, Func: crypto.ECDSASecp256r1Verify,
		Price: crypto.ECDSAVerifyPrice, ParamCount: 3},
	{Name: interopnames.NeoCryptoVerifyWithECDsaSecp256k1, Func: crypto.ECDSASecp256k1Verify,
		Price: crypto.ECDSAVerifyPrice, ParamCount: 3},
	{Name: interopnames.NeoCryptoCheckMultisigWithECDsaSecp256r1, Func: crypto.ECDSASecp256r1CheckMultisig, Price: 0, ParamCount: 3},
	{Name: interopnames.NeoCryptoCheckMultisigWithECDsaSecp256k1, Func: crypto.ECDSASecp256k1CheckMultisig, Price: 0, ParamCount: 3},
	{Name: interopnames.NeoCryptoSHA256, Func: crypto.Sha256, Price: 1000000, ParamCount: 1},
	{Name: interopnames.NeoCryptoRIPEMD160, Func: crypto.RipeMD160, Price: 1000000, ParamCount: 1},
	{Name: interopnames.NeoNativeCall, Func: native.Call, Price: 0, ParamCount: 1, DisallowCallback: true},
	{Name: interopnames.NeoNativeDeploy, Func: native.Deploy, Price: 0, RequiredFlags: smartcontract.AllowModifyStates, DisallowCallback: true},
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
