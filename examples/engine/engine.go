package enginecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// Main is that famous Main() function, you know.
func Main() bool {
	tx := runtime.GetScriptContainer()
	runtime.Notify("Tx", tx.Hash)

	callingScriptHash := runtime.GetCallingScriptHash()
	runtime.Notify("Calling", callingScriptHash)

	execScriptHash := runtime.GetExecutingScriptHash()
	runtime.Notify("Executing", execScriptHash)

	entryScriptHash := runtime.GetEntryScriptHash()
	runtime.Notify("Entry", entryScriptHash)

	return true
}
