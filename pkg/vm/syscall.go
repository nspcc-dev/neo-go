package vm

// Syscalls is a mapping between the syscall function name
// and the registerd VM interop API.
var Syscalls = map[string]string{
	// Storage API
	"GetContext": "Neo.Storage.GetContext",
	"Put":        "Neo.Storage.Put",
	"GetInt":     "Neo.Storage.Get",
	"GetString":  "Neo.Storage.Get",
	"Delete":     "Neo.Storage.Delete",
}
