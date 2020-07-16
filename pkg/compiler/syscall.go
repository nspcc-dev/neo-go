package compiler

// Syscall represents NEO or System syscall API with flag for proper AVM generation
type Syscall struct {
	API                   string
	ConvertResultToStruct bool
}

// All lists are sorted, keep 'em this way, please.
var syscalls = map[string]map[string]Syscall{
	"binary": {
		"Deserialize": {"System.Binary.Deserialize", false},
		"Serialize":   {"System.Binary.Serialize", false},
	},
	"blockchain": {
		"GetBlock":                {"System.Blockchain.GetBlock", true},
		"GetContract":             {"System.Blockchain.GetContract", false},
		"GetHeight":               {"System.Blockchain.GetHeight", false},
		"GetTransaction":          {"System.Blockchain.GetTransaction", true},
		"GetTransactionFromBlock": {"System.Blockchain.GetTransactionFromBlock", false},
		"GetTransactionHeight":    {"System.Blockchain.GetTransactionHeight", false},
	},
	"contract": {
		"Create":                {"System.Contract.Create", false},
		"CreateStandardAccount": {"System.Contract.CreateStandardAccount", false},
		"Destroy":               {"System.Contract.Destroy", false},
		"IsStandard":            {"System.Contract.IsStandard", false},
		"Update":                {"System.Contract.Update", false},
	},
	"crypto": {
		"ECDsaSecp256k1Verify":        {"Neo.Crypto.VerifyWithECDsaSecp256k1", false},
		"ECDSASecp256k1CheckMultisig": {"Neo.Crypto.CheckMultisigWithECDsaSecp256k1", false},
		"ECDsaSecp256r1Verify":        {"Neo.Crypto.VerifyWithECDsaSecp256r1", false},
		"ECDSASecp256r1CheckMultisig": {"Neo.Crypto.CheckMultisigWithECDsaSecp256r1", false},
	},
	"enumerator": {
		"Concat": {"System.Enumerator.Concat", false},
		"Create": {"System.Enumerator.Create", false},
		"Next":   {"System.Enumerator.Next", false},
		"Value":  {"System.Enumerator.Value", false},
	},
	"iterator": {
		"Concat": {"System.Iterator.Concat", false},
		"Create": {"System.Iterator.Create", false},
		"Key":    {"System.Iterator.Key", false},
		"Keys":   {"System.Iterator.Keys", false},
		"Next":   {"System.Enumerator.Next", false},
		"Value":  {"System.Enumerator.Value", false},
		"Values": {"System.Iterator.Values", false},
	},
	"json": {
		"Deserialize": {"System.Json.Deserialize", false},
		"Serialize":   {"System.Json.Serialize", false},
	},
	"runtime": {
		"GasLeft":                {"System.Runtime.GasLeft", false},
		"GetInvocationCounter":   {"System.Runtime.GetInvocationCounter", false},
		"GetCallingScriptHash":   {"System.Runtime.GetCallingScriptHash", false},
		"GetEntryScriptHash":     {"System.Runtime.GetEntryScriptHash", false},
		"GetExecutingScriptHash": {"System.Runtime.GetExecutingScriptHash", false},
		"GetNotifications":       {"System.Runtime.GetNotifications", false},
		"GetScriptContainer":     {"System.Runtime.GetScriptContainer", true},
		"GetTime":                {"System.Runtime.GetTime", false},
		"GetTrigger":             {"System.Runtime.GetTrigger", false},
		"CheckWitness":           {"System.Runtime.CheckWitness", false},
		"Log":                    {"System.Runtime.Log", false},
		"Notify":                 {"System.Runtime.Notify", false},
	},
	"storage": {
		"ConvertContextToReadOnly": {"System.Storage.AsReadOnly", false},
		"Delete":                   {"System.Storage.Delete", false},
		"Find":                     {"System.Storage.Find", false},
		"Get":                      {"System.Storage.Get", false},
		"GetContext":               {"System.Storage.GetContext", false},
		"GetReadOnlyContext":       {"System.Storage.GetReadOnlyContext", false},
		"Put":                      {"System.Storage.Put", false},
	},
}
