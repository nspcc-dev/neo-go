package compiler

import "github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"

// All lists are sorted, keep 'em this way, please.
var syscalls = map[string]map[string]string{
	"binary": {
		"Atoi":         interopnames.SystemBinaryAtoi,
		"Base58Decode": interopnames.SystemBinaryBase58Decode,
		"Base58Encode": interopnames.SystemBinaryBase58Encode,
		"Base64Decode": interopnames.SystemBinaryBase64Decode,
		"Base64Encode": interopnames.SystemBinaryBase64Encode,
		"Deserialize":  interopnames.SystemBinaryDeserialize,
		"Itoa":         interopnames.SystemBinaryItoa,
		"Serialize":    interopnames.SystemBinarySerialize,
	},
	"blockchain": {
		"GetBlock":                interopnames.SystemBlockchainGetBlock,
		"GetHeight":               interopnames.SystemBlockchainGetHeight,
		"GetTransaction":          interopnames.SystemBlockchainGetTransaction,
		"GetTransactionFromBlock": interopnames.SystemBlockchainGetTransactionFromBlock,
		"GetTransactionHeight":    interopnames.SystemBlockchainGetTransactionHeight,
	},
	"contract": {
		"Call":                  interopnames.SystemContractCall,
		"CreateStandardAccount": interopnames.SystemContractCreateStandardAccount,
		"IsStandard":            interopnames.SystemContractIsStandard,
		"GetCallFlags":          interopnames.SystemContractGetCallFlags,
	},
	"crypto": {
		"ECDsaSecp256k1Verify":        interopnames.NeoCryptoVerifyWithECDsaSecp256k1,
		"ECDSASecp256k1CheckMultisig": interopnames.NeoCryptoCheckMultisigWithECDsaSecp256k1,
		"ECDsaSecp256r1Verify":        interopnames.NeoCryptoVerifyWithECDsaSecp256r1,
		"ECDSASecp256r1CheckMultisig": interopnames.NeoCryptoCheckMultisigWithECDsaSecp256r1,
		"RIPEMD160":                   interopnames.NeoCryptoRIPEMD160,
		"SHA256":                      interopnames.NeoCryptoSHA256,
	},
	"enumerator": {
		"Create": interopnames.SystemEnumeratorCreate,
		"Next":   interopnames.SystemEnumeratorNext,
		"Value":  interopnames.SystemEnumeratorValue,
	},
	"iterator": {
		"Create": interopnames.SystemIteratorCreate,
		"Key":    interopnames.SystemIteratorKey,
		"Keys":   interopnames.SystemIteratorKeys,
		"Next":   interopnames.SystemEnumeratorNext,
		"Value":  interopnames.SystemEnumeratorValue,
		"Values": interopnames.SystemIteratorValues,
	},
	"json": {
		"Deserialize": interopnames.SystemJSONDeserialize,
		"Serialize":   interopnames.SystemJSONSerialize,
	},
	"runtime": {
		"GasLeft":                interopnames.SystemRuntimeGasLeft,
		"GetInvocationCounter":   interopnames.SystemRuntimeGetInvocationCounter,
		"GetCallingScriptHash":   interopnames.SystemRuntimeGetCallingScriptHash,
		"GetEntryScriptHash":     interopnames.SystemRuntimeGetEntryScriptHash,
		"GetExecutingScriptHash": interopnames.SystemRuntimeGetExecutingScriptHash,
		"GetNotifications":       interopnames.SystemRuntimeGetNotifications,
		"GetScriptContainer":     interopnames.SystemRuntimeGetScriptContainer,
		"GetTime":                interopnames.SystemRuntimeGetTime,
		"GetTrigger":             interopnames.SystemRuntimeGetTrigger,
		"CheckWitness":           interopnames.SystemRuntimeCheckWitness,
		"Log":                    interopnames.SystemRuntimeLog,
		"Notify":                 interopnames.SystemRuntimeNotify,
		"Platform":               interopnames.SystemRuntimePlatform,
	},
	"storage": {
		"ConvertContextToReadOnly": interopnames.SystemStorageAsReadOnly,
		"Delete":                   interopnames.SystemStorageDelete,
		"Find":                     interopnames.SystemStorageFind,
		"Get":                      interopnames.SystemStorageGet,
		"GetContext":               interopnames.SystemStorageGetContext,
		"GetReadOnlyContext":       interopnames.SystemStorageGetReadOnlyContext,
		"Put":                      interopnames.SystemStoragePut,
		"PutEx":                    interopnames.SystemStoragePutEx,
	},
}
