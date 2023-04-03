package oraclecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/oracle"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// RequestURL accepts a complete set of parameters to make an oracle request and
// performs it.
func RequestURL(url string, filter []byte, callback string, userData any, gasForResponse int) {
	oracle.Request(url, filter, callback, userData, gasForResponse)
}

// Handle is a response handler that writes response data to the storage.
func Handle(url string, data any, code int, res []byte) {
	// ABORT if len(data) == 2, some tests use this feature.
	if data != nil && len(data.(string)) == 2 {
		util.Abort()
	}
	params := []any{url, data, code, res}
	storage.Put(storage.GetContext(), "lastOracleResponse", std.Serialize(params))
}

// HandleRecursive invokes oracle.finish again to test Oracle reentrance.
func HandleRecursive(url string, data any, code int, res []byte) {
	// Regular safety check.
	callingHash := runtime.GetCallingScriptHash()
	if !callingHash.Equals(oracle.Hash) {
		panic("not called from oracle contract")
	}

	runtime.Notify("Invocation")
	if runtime.GetInvocationCounter() == 1 {
		// We provide no wrapper for finish in interops, it's not usually needed.
		contract.Call(interop.Hash160(oracle.Hash), "finish", contract.All)
	}
}
