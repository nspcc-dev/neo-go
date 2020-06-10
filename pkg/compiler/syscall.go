package compiler

var syscalls = map[string]map[string]string{
	"crypto": {
		"ECDsaVerify": "Neo.Crypto.ECDsaVerify",
	},
	"enumerator": {
		"Concat": "Neo.Enumerator.Concat",
		"Create": "Neo.Enumerator.Create",
		"Next":   "Neo.Enumerator.Next",
		"Value":  "Neo.Enumerator.Value",
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
		"GetTrigger":   "Neo.Runtime.GetTrigger",
		"CheckWitness": "Neo.Runtime.CheckWitness",
		"Notify":       "Neo.Runtime.Notify",
		"Log":          "Neo.Runtime.Log",
		"GetTime":      "Neo.Runtime.GetTime",
		"Serialize":    "Neo.Runtime.Serialize",
		"Deserialize":  "Neo.Runtime.Deserialize",
	},
	"blockchain": {
		"GetBlock":                "System.Blockchain.GetBlock",
		"GetContract":             "Neo.Blockchain.GetContract",
		"GetHeight":               "Neo.Blockchain.GetHeight",
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
		"Concat": "Neo.Iterator.Concat",
		"Create": "Neo.Iterator.Create",
		"Key":    "Neo.Iterator.Key",
		"Keys":   "Neo.Iterator.Keys",
		"Next":   "Neo.Iterator.Next",
		"Value":  "Neo.Iterator.Value",
		"Values": "Neo.Iterator.Values",
	},
}
