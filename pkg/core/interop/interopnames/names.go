package interopnames

// Names of all used interops.
const (
	SystemCallbackCreate                = "System.Callback.Create"
	SystemCallbackCreateFromMethod      = "System.Callback.CreateFromMethod"
	SystemCallbackCreateFromSyscall     = "System.Callback.CreateFromSyscall"
	SystemCallbackInvoke                = "System.Callback.Invoke"
	SystemContractCall                  = "System.Contract.Call"
	SystemContractCallNative            = "System.Contract.CallNative"
	SystemContractCreateMultisigAccount = "System.Contract.CreateMultisigAccount"
	SystemContractCreateStandardAccount = "System.Contract.CreateStandardAccount"
	SystemContractGetCallFlags          = "System.Contract.GetCallFlags"
	SystemContractNativeOnPersist       = "System.Contract.NativeOnPersist"
	SystemContractNativePostPersist     = "System.Contract.NativePostPersist"
	SystemIteratorNext                  = "System.Iterator.Next"
	SystemIteratorValue                 = "System.Iterator.Value"
	SystemRuntimeBurnGas                = "System.Runtime.BurnGas"
	SystemRuntimeCheckWitness           = "System.Runtime.CheckWitness"
	SystemRuntimeGasLeft                = "System.Runtime.GasLeft"
	SystemRuntimeGetCallingScriptHash   = "System.Runtime.GetCallingScriptHash"
	SystemRuntimeGetEntryScriptHash     = "System.Runtime.GetEntryScriptHash"
	SystemRuntimeGetExecutingScriptHash = "System.Runtime.GetExecutingScriptHash"
	SystemRuntimeGetInvocationCounter   = "System.Runtime.GetInvocationCounter"
	SystemRuntimeGetNotifications       = "System.Runtime.GetNotifications"
	SystemRuntimeGetScriptContainer     = "System.Runtime.GetScriptContainer"
	SystemRuntimeGetTime                = "System.Runtime.GetTime"
	SystemRuntimeGetTrigger             = "System.Runtime.GetTrigger"
	SystemRuntimeLog                    = "System.Runtime.Log"
	SystemRuntimeNotify                 = "System.Runtime.Notify"
	SystemRuntimePlatform               = "System.Runtime.Platform"
	SystemStorageDelete                 = "System.Storage.Delete"
	SystemStorageFind                   = "System.Storage.Find"
	SystemStorageGet                    = "System.Storage.Get"
	SystemStorageGetContext             = "System.Storage.GetContext"
	SystemStorageGetReadOnlyContext     = "System.Storage.GetReadOnlyContext"
	SystemStoragePut                    = "System.Storage.Put"
	SystemStorageAsReadOnly             = "System.Storage.AsReadOnly"
	NeoCryptoCheckMultisig              = "Neo.Crypto.CheckMultisig"
	NeoCryptoCheckSig                   = "Neo.Crypto.CheckSig"
)

var names = []string{
	SystemCallbackCreate,
	SystemCallbackCreateFromMethod,
	SystemCallbackCreateFromSyscall,
	SystemCallbackInvoke,
	SystemContractCall,
	SystemContractCallNative,
	SystemContractCreateMultisigAccount,
	SystemContractCreateStandardAccount,
	SystemContractGetCallFlags,
	SystemContractNativeOnPersist,
	SystemContractNativePostPersist,
	SystemIteratorNext,
	SystemIteratorValue,
	SystemRuntimeBurnGas,
	SystemRuntimeCheckWitness,
	SystemRuntimeGasLeft,
	SystemRuntimeGetCallingScriptHash,
	SystemRuntimeGetEntryScriptHash,
	SystemRuntimeGetExecutingScriptHash,
	SystemRuntimeGetInvocationCounter,
	SystemRuntimeGetNotifications,
	SystemRuntimeGetScriptContainer,
	SystemRuntimeGetTime,
	SystemRuntimeGetTrigger,
	SystemRuntimeLog,
	SystemRuntimeNotify,
	SystemRuntimePlatform,
	SystemStorageDelete,
	SystemStorageFind,
	SystemStorageGet,
	SystemStorageGetContext,
	SystemStorageGetReadOnlyContext,
	SystemStoragePut,
	SystemStorageAsReadOnly,
	NeoCryptoCheckMultisig,
	NeoCryptoCheckSig,
}
