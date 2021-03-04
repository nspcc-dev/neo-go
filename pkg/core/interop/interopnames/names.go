package interopnames

// Names of all used interops.
const (
	SystemBinaryBase58Decode                 = "System.Binary.Base58Decode"
	SystemBinaryBase58Encode                 = "System.Binary.Base58Encode"
	SystemBinaryBase64Decode                 = "System.Binary.Base64Decode"
	SystemBinaryBase64Encode                 = "System.Binary.Base64Encode"
	SystemBinaryDeserialize                  = "System.Binary.Deserialize"
	SystemBinarySerialize                    = "System.Binary.Serialize"
	SystemCallbackCreate                     = "System.Callback.Create"
	SystemCallbackCreateFromMethod           = "System.Callback.CreateFromMethod"
	SystemCallbackCreateFromSyscall          = "System.Callback.CreateFromSyscall"
	SystemCallbackInvoke                     = "System.Callback.Invoke"
	SystemContractCall                       = "System.Contract.Call"
	SystemContractCallNative                 = "System.Contract.CallNative"
	SystemContractCreateMultisigAccount      = "System.Contract.CreateMultisigAccount"
	SystemContractCreateStandardAccount      = "System.Contract.CreateStandardAccount"
	SystemContractIsStandard                 = "System.Contract.IsStandard"
	SystemContractGetCallFlags               = "System.Contract.GetCallFlags"
	SystemContractNativeOnPersist            = "System.Contract.NativeOnPersist"
	SystemContractNativePostPersist          = "System.Contract.NativePostPersist"
	SystemIteratorCreate                     = "System.Iterator.Create"
	SystemIteratorNext                       = "System.Iterator.Next"
	SystemIteratorValue                      = "System.Iterator.Value"
	SystemJSONDeserialize                    = "System.Json.Deserialize"
	SystemJSONSerialize                      = "System.Json.Serialize"
	SystemRuntimeCheckWitness                = "System.Runtime.CheckWitness"
	SystemRuntimeGasLeft                     = "System.Runtime.GasLeft"
	SystemRuntimeGetCallingScriptHash        = "System.Runtime.GetCallingScriptHash"
	SystemRuntimeGetEntryScriptHash          = "System.Runtime.GetEntryScriptHash"
	SystemRuntimeGetExecutingScriptHash      = "System.Runtime.GetExecutingScriptHash"
	SystemRuntimeGetInvocationCounter        = "System.Runtime.GetInvocationCounter"
	SystemRuntimeGetNotifications            = "System.Runtime.GetNotifications"
	SystemRuntimeGetScriptContainer          = "System.Runtime.GetScriptContainer"
	SystemRuntimeGetTime                     = "System.Runtime.GetTime"
	SystemRuntimeGetTrigger                  = "System.Runtime.GetTrigger"
	SystemRuntimeLog                         = "System.Runtime.Log"
	SystemRuntimeNotify                      = "System.Runtime.Notify"
	SystemRuntimePlatform                    = "System.Runtime.Platform"
	SystemStorageDelete                      = "System.Storage.Delete"
	SystemStorageFind                        = "System.Storage.Find"
	SystemStorageGet                         = "System.Storage.Get"
	SystemStorageGetContext                  = "System.Storage.GetContext"
	SystemStorageGetReadOnlyContext          = "System.Storage.GetReadOnlyContext"
	SystemStoragePut                         = "System.Storage.Put"
	SystemStorageAsReadOnly                  = "System.Storage.AsReadOnly"
	NeoCryptoVerifyWithECDsaSecp256r1        = "Neo.Crypto.VerifyWithECDsaSecp256r1"
	NeoCryptoVerifyWithECDsaSecp256k1        = "Neo.Crypto.VerifyWithECDsaSecp256k1"
	NeoCryptoCheckMultisigWithECDsaSecp256r1 = "Neo.Crypto.CheckMultisigWithECDsaSecp256r1"
	NeoCryptoCheckMultisigWithECDsaSecp256k1 = "Neo.Crypto.CheckMultisigWithECDsaSecp256k1"
)

var names = []string{
	SystemBinaryBase58Decode,
	SystemBinaryBase58Encode,
	SystemBinaryBase64Decode,
	SystemBinaryBase64Encode,
	SystemBinaryDeserialize,
	SystemBinarySerialize,
	SystemCallbackCreate,
	SystemCallbackCreateFromMethod,
	SystemCallbackCreateFromSyscall,
	SystemCallbackInvoke,
	SystemContractCall,
	SystemContractCallNative,
	SystemContractCreateMultisigAccount,
	SystemContractCreateStandardAccount,
	SystemContractIsStandard,
	SystemContractGetCallFlags,
	SystemContractNativeOnPersist,
	SystemContractNativePostPersist,
	SystemIteratorCreate,
	SystemIteratorNext,
	SystemIteratorValue,
	SystemJSONDeserialize,
	SystemJSONSerialize,
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
	NeoCryptoVerifyWithECDsaSecp256r1,
	NeoCryptoVerifyWithECDsaSecp256k1,
	NeoCryptoCheckMultisigWithECDsaSecp256r1,
	NeoCryptoCheckMultisigWithECDsaSecp256k1,
}
