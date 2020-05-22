package enginecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/engine"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// Main is that famous Main() function, you know.
func Main() bool {
	tx := engine.GetScriptContainer()
	runtime.Notify(tx)

	callingScriptHash := engine.GetCallingScriptHash()
	runtime.Notify(callingScriptHash)

	execScriptHash := engine.GetExecutingScriptHash()
	runtime.Notify(execScriptHash)

	entryScriptHash := engine.GetEntryScriptHash()
	runtime.Notify(entryScriptHash)

	return true
}
