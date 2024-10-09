package enginecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// NotifyScriptContainer sends runtime notification with script container hash.
func NotifyScriptContainer() {
	tx := runtime.GetScriptContainer()
	runtime.Notify("Tx", tx.Hash)
}

// NotifyCallingScriptHash sends runtime notification with calling script hash.
func NotifyCallingScriptHash() {
	callingScriptHash := runtime.GetCallingScriptHash()
	runtime.Notify("Calling", callingScriptHash)
}

// NotifyExecutingScriptHash sends runtime notification about executing script hash.
func NotifyExecutingScriptHash() {
	execScriptHash := runtime.GetExecutingScriptHash()
	runtime.Notify("Executing", execScriptHash)
}

// NotifyEntryScriptHash sends notification about entry script hash.
func NotifyEntryScriptHash() {
	entryScriptHash := runtime.GetEntryScriptHash()
	runtime.Notify("Entry", entryScriptHash)
}
