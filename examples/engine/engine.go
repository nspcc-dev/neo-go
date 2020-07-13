package enginecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// Main is that famous Main() function, you know.
func Main() bool {
	tx := runtime.GetScriptContainer()
	runtime.Notify(tx.Hash)

	callingScriptHash := runtime.GetCallingScriptHash()
	runtime.Notify(callingScriptHash)

	execScriptHash := runtime.GetExecutingScriptHash()
	runtime.Notify(execScriptHash)

	entryScriptHash := runtime.GetEntryScriptHash()
	runtime.Notify(entryScriptHash)

	return true
}
