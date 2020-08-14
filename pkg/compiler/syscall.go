package compiler

import "github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"

// Syscall represents NEO or System syscall API with flag for proper AVM generation
type Syscall struct {
	API                   string
	ConvertResultToStruct bool
}

// All lists are sorted, keep 'em this way, please.
var syscalls = map[string]map[string]Syscall{
	"binary": {
		"Base64Decode": {interopnames.SystemBinaryBase64Decode, false},
		"Base64Encode": {interopnames.SystemBinaryBase64Encode, false},
		"Deserialize":  {interopnames.SystemBinaryDeserialize, false},
		"Serialize":    {interopnames.SystemBinarySerialize, false},
	},
	"blockchain": {
		"GetBlock":                {interopnames.SystemBlockchainGetBlock, true},
		"GetContract":             {interopnames.SystemBlockchainGetContract, true},
		"GetHeight":               {interopnames.SystemBlockchainGetHeight, false},
		"GetTransaction":          {interopnames.SystemBlockchainGetTransaction, true},
		"GetTransactionFromBlock": {interopnames.SystemBlockchainGetTransactionFromBlock, false},
		"GetTransactionHeight":    {interopnames.SystemBlockchainGetTransactionHeight, false},
	},
	"contract": {
		"Create":                {interopnames.SystemContractCreate, true},
		"CreateStandardAccount": {interopnames.SystemContractCreateStandardAccount, false},
		"Destroy":               {interopnames.SystemContractDestroy, false},
		"IsStandard":            {interopnames.SystemContractIsStandard, false},
		"GetCallFlags":          {interopnames.SystemContractGetCallFlags, false},
		"Update":                {interopnames.SystemContractUpdate, false},
	},
	"crypto": {
		"ECDsaSecp256k1Verify":        {interopnames.NeoCryptoVerifyWithECDsaSecp256k1, false},
		"ECDSASecp256k1CheckMultisig": {interopnames.NeoCryptoCheckMultisigWithECDsaSecp256k1, false},
		"ECDsaSecp256r1Verify":        {interopnames.NeoCryptoVerifyWithECDsaSecp256r1, false},
		"ECDSASecp256r1CheckMultisig": {interopnames.NeoCryptoCheckMultisigWithECDsaSecp256r1, false},
		"RIPEMD160":                   {interopnames.NeoCryptoRIPEMD160, false},
		"SHA256":                      {interopnames.NeoCryptoSHA256, false},
	},
	"enumerator": {
		"Concat": {interopnames.SystemEnumeratorConcat, false},
		"Create": {interopnames.SystemEnumeratorCreate, false},
		"Next":   {interopnames.SystemEnumeratorNext, false},
		"Value":  {interopnames.SystemEnumeratorValue, false},
	},
	"engine": {
		"AppCall": {interopnames.SystemContractCall, false},
	},
	"iterator": {
		"Concat": {interopnames.SystemIteratorConcat, false},
		"Create": {interopnames.SystemIteratorCreate, false},
		"Key":    {interopnames.SystemIteratorKey, false},
		"Keys":   {interopnames.SystemIteratorKeys, false},
		"Next":   {interopnames.SystemEnumeratorNext, false},
		"Value":  {interopnames.SystemEnumeratorValue, false},
		"Values": {interopnames.SystemIteratorValues, false},
	},
	"json": {
		"Deserialize": {interopnames.SystemJSONDeserialize, false},
		"Serialize":   {interopnames.SystemJSONSerialize, false},
	},
	"runtime": {
		"GasLeft":                {interopnames.SystemRuntimeGasLeft, false},
		"GetInvocationCounter":   {interopnames.SystemRuntimeGetInvocationCounter, false},
		"GetCallingScriptHash":   {interopnames.SystemRuntimeGetCallingScriptHash, false},
		"GetEntryScriptHash":     {interopnames.SystemRuntimeGetEntryScriptHash, false},
		"GetExecutingScriptHash": {interopnames.SystemRuntimeGetExecutingScriptHash, false},
		"GetNotifications":       {interopnames.SystemRuntimeGetNotifications, false},
		"GetScriptContainer":     {interopnames.SystemRuntimeGetScriptContainer, true},
		"GetTime":                {interopnames.SystemRuntimeGetTime, false},
		"GetTrigger":             {interopnames.SystemRuntimeGetTrigger, false},
		"CheckWitness":           {interopnames.SystemRuntimeCheckWitness, false},
		"Log":                    {interopnames.SystemRuntimeLog, false},
		"Notify":                 {interopnames.SystemRuntimeNotify, false},
		"Platform":               {interopnames.SystemRuntimePlatform, false},
	},
	"storage": {
		"ConvertContextToReadOnly": {interopnames.SystemStorageAsReadOnly, false},
		"Delete":                   {interopnames.SystemStorageDelete, false},
		"Find":                     {interopnames.SystemStorageFind, false},
		"Get":                      {interopnames.SystemStorageGet, false},
		"GetContext":               {interopnames.SystemStorageGetContext, false},
		"GetReadOnlyContext":       {interopnames.SystemStorageGetReadOnlyContext, false},
		"Put":                      {interopnames.SystemStoragePut, false},
		"PutEx":                    {interopnames.SystemStoragePutEx, false},
	},
}
