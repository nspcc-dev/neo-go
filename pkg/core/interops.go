package core

/*
  Interops are designed to run under VM's execute() panic protection, so it's OK
  for them to do things like
          smth := v.Estack().Pop().Bytes()
  even though technically Pop() can return a nil pointer.
*/

import (
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/config"
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
	{Name: interopnames.SystemContractCall, Func: contract.Call,
		RequiredFlags: callflag.ReadStates | callflag.AllowCall, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 0}}},
	{Name: interopnames.SystemContractCallNative, Func: native.Call,
		Prices: []interop.HFPrice{{Hardfork: config.HFHuyao, Price: 1130000}}},
	{Name: interopnames.SystemContractCreateMultisigAccount, Func: contract.CreateMultisigAccount},
	{Name: interopnames.SystemContractCreateStandardAccount, Func: contract.CreateStandardAccount,
		Prices: []interop.HFPrice{{Hardfork: config.HFHuyao, Price: 41833}}},
	{Name: interopnames.SystemContractGetCallFlags, Func: contract.GetCallFlags,
		Prices: []interop.HFPrice{{Price: 1 << 10 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 5933}}},
	{Name: interopnames.SystemContractNativeOnPersist, Func: native.OnPersist,
		RequiredFlags: callflag.States},
	{Name: interopnames.SystemContractNativePostPersist, Func: native.PostPersist,
		RequiredFlags: callflag.States},
	{Name: interopnames.SystemCryptoCheckMultisig, Func: crypto.ECDSASecp256r1CheckMultisig},
	{Name: interopnames.SystemCryptoCheckSig, Func: crypto.ECDSASecp256r1CheckSig,
		Prices: []interop.HFPrice{{Price: fee.ECDSAVerifyPrice * vm.OpcodePriceMultiplier},
			{Hardfork: config.HFHuyao, Price: fee.ECDSAVerifyPriceAfterHuyao}}},
	{Name: interopnames.SystemIteratorNext, Func: iterator.Next,
		Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 3367}}},
	{Name: interopnames.SystemIteratorValue, Func: iterator.Value,
		Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 0}}},
	{Name: interopnames.SystemRuntimeBurnGas, Func: runtime.BurnGas,
		Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 3067}}},
	{Name: interopnames.SystemRuntimeCheckWitness, Func: runtime.CheckWitness,
		RequiredFlags: callflag.NoneFlag, Prices: []interop.HFPrice{{Price: 1 << 10 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 0}}},
	{Name: interopnames.SystemRuntimeCurrentSigners, Func: runtime.CurrentSigners,
		RequiredFlags: callflag.NoneFlag, Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 0}}},
	{Name: interopnames.SystemRuntimeGasLeft, Func: runtime.GasLeft,
		Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 8900}}},
	{Name: interopnames.SystemRuntimeGetAddressVersion, Func: runtime.GetAddressVersion,
		Prices: []interop.HFPrice{{Price: 1 << 3 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 8867}}},
	{Name: interopnames.SystemRuntimeGetCallingScriptHash, Func: runtime.GetCallingScriptHash,
		Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 9567}}},
	{Name: interopnames.SystemRuntimeGetEntryScriptHash, Func: runtime.GetEntryScriptHash,
		Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 9900}}},
	{Name: interopnames.SystemRuntimeGetExecutingScriptHash, Func: runtime.GetExecutingScriptHash,
		Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 9900}}},
	{Name: interopnames.SystemRuntimeGetInvocationCounter, Func: runtime.GetInvocationCounter,
		Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 10267}}},
	{Name: interopnames.SystemRuntimeGetNetwork, Func: runtime.GetNetwork,
		Prices: []interop.HFPrice{{Price: 1 << 3 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 9833}}},
	{Name: interopnames.SystemRuntimeGetNotifications, Func: runtime.GetNotifications,
		Prices: []interop.HFPrice{{Price: 1 << 12 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 0}}},
	{Name: interopnames.SystemRuntimeGetRandom, Func: runtime.GetRandom,
		Prices: []interop.HFPrice{{Hardfork: config.HFHuyao, Price: 15833}}},
	{Name: interopnames.SystemRuntimeGetScriptContainer, Func: runtime.GetScriptContainer,
		Prices: []interop.HFPrice{{Price: 1 << 3 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 36533}}},
	{Name: interopnames.SystemRuntimeGetTime, Func: runtime.GetTime,
		RequiredFlags: callflag.ReadStates, Prices: []interop.HFPrice{{Price: 1 << 3 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 9033}}},
	{Name: interopnames.SystemRuntimeGetTrigger, Func: runtime.GetTrigger,
		Prices: []interop.HFPrice{{Price: 1 << 3 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 8700}}},
	{Name: interopnames.SystemRuntimeLoadScript, Func: runtime.LoadScript,
		RequiredFlags: callflag.AllowCall, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 0}}},
	{Name: interopnames.SystemRuntimeLog, Func: runtime.Log,
		RequiredFlags: callflag.AllowNotify, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 31767}}},
	{Name: interopnames.SystemRuntimeNotify, Func: runtime.Notify,
		RequiredFlags: callflag.AllowNotify, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 0}}},
	{Name: interopnames.SystemRuntimePlatform, Func: runtime.Platform,
		Prices: []interop.HFPrice{{Price: 1 << 3 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 8867}}},
	{Name: interopnames.SystemStorageDelete, Func: storage.Delete,
		RequiredFlags: callflag.WriteStates, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}}},
	{Name: interopnames.SystemStorageFind, Func: storage.Find,
		RequiredFlags: callflag.ReadStates, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: fee.ReadFromDiskPrice}}},
	{Name: interopnames.SystemStorageGet, Func: storage.Get,
		RequiredFlags: callflag.ReadStates, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: fee.ReadFromDiskPrice}}},
	{Name: interopnames.SystemStorageGetContext, Func: storage.GetContext,
		RequiredFlags: callflag.ReadStates, Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 12200}}},
	{Name: interopnames.SystemStorageGetReadOnlyContext, Func: storage.GetReadOnlyContext,
		RequiredFlags: callflag.ReadStates, Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 11933}}},
	{Name: interopnames.SystemStoragePut, Func: storage.Put,
		RequiredFlags: callflag.WriteStates, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}}},
	{Name: interopnames.SystemStorageAsReadOnly, Func: storage.ContextAsReadOnly,
		RequiredFlags: callflag.ReadStates, Prices: []interop.HFPrice{{Price: 1 << 4 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: 11233}}},
	{Name: interopnames.SystemStorageLocalGet, Func: storage.LocalGet,
		RequiredFlags: callflag.ReadStates, ActiveFrom: config.HFFaun, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: fee.ReadFromDiskPrice}}},
	{Name: interopnames.SystemStorageLocalFind, Func: storage.LocalFind,
		RequiredFlags: callflag.ReadStates, ActiveFrom: config.HFFaun, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}, {Hardfork: config.HFHuyao, Price: fee.ReadFromDiskPrice}}},
	{Name: interopnames.SystemStorageLocalPut, Func: storage.LocalPut,
		RequiredFlags: callflag.WriteStates, ActiveFrom: config.HFFaun, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}}},
	{Name: interopnames.SystemStorageLocalDelete, Func: storage.LocalDelete,
		RequiredFlags: callflag.WriteStates, ActiveFrom: config.HFFaun, Prices: []interop.HFPrice{{Price: 1 << 15 * vm.OpcodePriceMultiplier}}},
}

// init initializes IDs in the global interop slices and sorts interop prices by
// hardfork in descending order. It panics if some interop has several prices
// for the same hardfork.
func init() {
	for i := range systemInterops {
		systemInterops[i].ID = interopnames.ToID([]byte(systemInterops[i].Name))
		prices := systemInterops[i].Prices
		slices.SortFunc(prices, func(a, b interop.HFPrice) int { return b.Hardfork.Cmp(a.Hardfork) })
		for j := range len(prices) - 1 {
			if prices[j].Hardfork == prices[j+1].Hardfork {
				panic(fmt.Sprintf("duplicating %s price for %s", prices[j].Hardfork, systemInterops[i].Name))
			}
		}
	}
	interop.Sort(systemInterops)
}
