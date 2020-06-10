package compiler

var syscalls = map[string]map[string]string{
	"crypto": {
		"ECDsaVerify": "Neo.Crypto.ECDsaVerify",
	},
	"enumerator": {
		"Concat": "System.Enumerator.Concat",
		"Create": "System.Enumerator.Create",
		"Next":   "System.Enumerator.Next",
		"Value":  "System.Enumerator.Value",
	},
	"storage": {
		"ConvertContextToReadOnly": "Neo.StorageContext.AsReadOnly",
		"Delete":                   "Neo.Storage.Delete",
		"Find":                     "Neo.Storage.Find",
		"Get":                      "Neo.Storage.Get",
		"GetContext":               "Neo.Storage.GetContext",
		"GetReadOnlyContext":       "Neo.Storage.GetReadOnlyContext",
		"Put":                      "Neo.Storage.Put",
	},
	"runtime": {
		"GetTrigger":   "System.Runtime.GetTrigger",
		"CheckWitness": "System.Runtime.CheckWitness",
		"Notify":       "System.Runtime.Notify",
		"Log":          "System.Runtime.Log",
		"GetTime":      "System.Runtime.GetTime",
		"Serialize":    "System.Runtime.Serialize",
		"Deserialize":  "System.Runtime.Deserialize",
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
		"GetScript":         "Neo.Contract.GetScript",
		"IsPayable":         "Neo.Contract.IsPayable",
		"Create":            "Neo.Contract.Create",
		"Destroy":           "Neo.Contract.Destroy",
		"Migrate":           "Neo.Contract.Migrate",
		"GetStorageContext": "Neo.Contract.GetStorageContext",
	},
	"engine": {
		"GetScriptContainer":     "System.ExecutionEngine.GetScriptContainer",
		"GetCallingScriptHash":   "System.ExecutionEngine.GetCallingScriptHash",
		"GetEntryScriptHash":     "System.ExecutionEngine.GetEntryScriptHash",
		"GetExecutingScriptHash": "System.ExecutionEngine.GetExecutingScriptHash",
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
