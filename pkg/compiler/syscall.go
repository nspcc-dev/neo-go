package compiler

var syscalls = map[string]map[string]string{
	"binary": {
		"Serialize":   "System.Binary.Serialize",
		"Deserialize": "System.Binary.Deserialize",
	},
	"crypto": {
		"ECDsaVerify": "Neo.Crypto.ECDsaVerify",
	},
	"enumerator": {
		"Concat": "System.Enumerator.Concat",
		"Create": "System.Enumerator.Create",
		"Next":   "System.Enumerator.Next",
		"Value":  "System.Enumerator.Value",
	},
	"json": {
		"Serialize":   "System.Json.Serialize",
		"Deserialize": "System.Json.Deserialize",
	},
	"storage": {
		"ConvertContextToReadOnly": "System.Storage.AsReadOnly",
		"Delete":                   "System.Storage.Delete",
		"Find":                     "System.Storage.Find",
		"Get":                      "System.Storage.Get",
		"GetContext":               "System.Storage.GetContext",
		"GetReadOnlyContext":       "System.Storage.GetReadOnlyContext",
		"Put":                      "System.Storage.Put",
	},
	"runtime": {
		"GetScriptContainer":     "System.Runtime.GetScriptContainer",
		"GetCallingScriptHash":   "System.Runtime.GetCallingScriptHash",
		"GetEntryScriptHash":     "System.Runtime.GetEntryScriptHash",
		"GetExecutingScriptHash": "System.Runtime.GetExecutingScriptHash",
		"GetNotifications":       "System.Runtime.GetNotifications",
		"GetInvocationCounter":   "System.Runtime.GetInvocationCounter",

		"GasLeft":      "System.Runtime.GasLeft",
		"GetTrigger":   "System.Runtime.GetTrigger",
		"CheckWitness": "System.Runtime.CheckWitness",
		"Notify":       "System.Runtime.Notify",
		"Log":          "System.Runtime.Log",
		"GetTime":      "System.Runtime.GetTime",
	},
	"blockchain": {
		"GetBlock":                "System.Blockchain.GetBlock",
		"GetContract":             "System.Blockchain.GetContract",
		"GetHeight":               "System.Blockchain.GetHeight",
		"GetTransaction":          "System.Blockchain.GetTransaction",
		"GetTransactionFromBlock": "System.Blockchain.GetTransactionFromBlock",
		"GetTransactionHeight":    "System.Blockchain.GetTransactionHeight",
	},
	"contract": {
		"Create":  "System.Contract.Create",
		"Destroy": "System.Contract.Destroy",
		"Update":  "System.Contract.Update",

		"IsStandard":            "System.Contract.IsStandard",
		"CreateStandardAccount": "System.Contract.CreateStandardAccount",
	},
	"iterator": {
		"Concat": "System.Iterator.Concat",
		"Create": "System.Iterator.Create",
		"Key":    "System.Iterator.Key",
		"Keys":   "System.Iterator.Keys",
		"Next":   "System.Enumerator.Next",
		"Value":  "System.Enumerator.Value",
		"Values": "System.Iterator.Values",
	},
}
