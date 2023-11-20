package interopnames

// Names of all used interops.
const (
	SystemContractCall                  = "System.Contract.Call"
	SystemContractCallNative            = "System.Contract.CallNative"
	SystemContractCreateMultisigAccount = "System.Contract.CreateMultisigAccount"
	SystemContractCreateStandardAccount = "System.Contract.CreateStandardAccount"
	SystemContractGetCallFlags          = "System.Contract.GetCallFlags"
	SystemContractNativeOnPersist       = "System.Contract.NativeOnPersist"
	SystemContractNativePostPersist     = "System.Contract.NativePostPersist"
	SystemCryptoCheckSig                = "System.Crypto.CheckSig"
	SystemCryptoCheckMultisig           = "System.Crypto.CheckMultisig"
	SystemIteratorNext                  = "System.Iterator.Next"
	SystemIteratorValue                 = "System.Iterator.Value"
	SystemRuntimeBurnGas                = "System.Runtime.BurnGas"
	SystemRuntimeCheckWitness           = "System.Runtime.CheckWitness"
	SystemRuntimeCurrentSigners         = "System.Runtime.CurrentSigners"
	SystemRuntimeGasLeft                = "System.Runtime.GasLeft"
	SystemRuntimeGetAddressVersion      = "System.Runtime.GetAddressVersion"
	SystemRuntimeGetCallingScriptHash   = "System.Runtime.GetCallingScriptHash"
	SystemRuntimeGetEntryScriptHash     = "System.Runtime.GetEntryScriptHash"
	SystemRuntimeGetExecutingScriptHash = "System.Runtime.GetExecutingScriptHash"
	SystemRuntimeGetInvocationCounter   = "System.Runtime.GetInvocationCounter"
	SystemRuntimeGetNetwork             = "System.Runtime.GetNetwork"
	SystemRuntimeGetNotifications       = "System.Runtime.GetNotifications"
	SystemRuntimeGetRandom              = "System.Runtime.GetRandom"
	SystemRuntimeGetScriptContainer     = "System.Runtime.GetScriptContainer"
	SystemRuntimeGetTime                = "System.Runtime.GetTime"
	SystemRuntimeGetTrigger             = "System.Runtime.GetTrigger"
	SystemRuntimeLoadScript             = "System.Runtime.LoadScript"
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
)

var names = []string{
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
	SystemRuntimeCurrentSigners,
	SystemRuntimeGasLeft,
	SystemRuntimeGetAddressVersion,
	SystemRuntimeGetCallingScriptHash,
	SystemRuntimeGetEntryScriptHash,
	SystemRuntimeGetExecutingScriptHash,
	SystemRuntimeGetInvocationCounter,
	SystemRuntimeGetNetwork,
	SystemRuntimeGetNotifications,
	SystemRuntimeGetRandom,
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
	SystemCryptoCheckMultisig,
	SystemCryptoCheckSig,
}
