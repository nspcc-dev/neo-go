package engine_contract

import (
	"github.com/CityOfZion/neo-storm/interop/engine"
	"github.com/CityOfZion/neo-storm/interop/runtime"
)

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
