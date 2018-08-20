package compiler

var syscalls = map[string]string{
	//
	// Standard library API
	//

	// Storage API
	"GetContext": "System.Storage.GetContext",
	"Put":        "System.Storage.Put",
	"Get":        "System.Storage.Get",
	"Delete":     "System.Storage.Delete",
	"Find":       "System.Storage.Find",

	// Runtime API
	"GetTrigger":   "System.Runtime.GetTrigger",
	"CheckWitness": "System.Runtime.CheckWitness",
	"Notify":       "System.Runtime.Notify",
	"Log":          "System.Runtime.Log",
	"GetTime":      "System.Runtime.GetTime",
	"Serialize":    "System.Runtime.Serialize",
	"Deserialize":  "System.Runtime.Deserialize",

	// Blockchain API
	"GetHeight":            "System.Blockchain.GetHeight",
	"GetHeader":            "System.Blockchain.GetHeader",
	"GetBlock":             "System.Blockchain.GetBlock",
	"GetTransaction":       "System.Blockchain.GetTransaction",
	"GetTransactionHeight": "System.Blockchain.GetTransactionHeight",
	"GetContract":          "System.Blockchain.GetContract",

	// Header API
	"GetIndex":     "System.Header.GetContract",
	"GetHash":      "System.Header.GetHash",
	"GetPrevHash":  "System.Header.GetPrevHash",
	"GetTimestamp": "System.Header.GetTimestamp",

	// Block API
	"GetTransactionCount": "System.Block.GetTransactionCount",
	"GetTransactions":     "System.Block.GetTransactions",
	// TODO: Find solution for duplicated map entry
	"NGetTransaction": "System.Block.GetTransaction",

	//
	// NEO specific API
	//

	// Blockchain API
	"GetAccount":    "Neo.Blockchain.GetAccount",
	"GetValidators": "Neo.Blockchain.GetValidators",
	"GetAsset":      "Neo.Blockchain.GetAsset",

	// Header API
	"GetVersion":       "Neo.Header.GetVersion",
	"GetMerkleRoot":    "Neo.Header.GetMerkleRoot",
	"GetConsensusData": "Neo.Header.GetConsensusData",
	"GetNextConsensus": "Neo.Header.GetNextConsensus",

	// Transaction API
	"GetType":         "Neo.Transaction.GetType",
	"GetAttributes":   "Neo.Transaction.GetAttributes",
	"GetInputs":       "Neo.Transaction.GetInputs",
	"GetOutputs":      "Neo.Transaction.GetOutputs",
	"GetReferences":   "Neo.Transaction.GetReferences",
	"GetUnspentCoins": "Neo.Transaction.GetUnspentCoins",
	"GetScript":       "Neo.InvocationTransaction.GetScript",

	// TODO: Add the rest of the interop APIS
}
