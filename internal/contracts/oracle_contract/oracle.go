package oraclecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/native/oracle"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// RequestURL accepts a complete set of parameters to make an oracle request and
// performs it.
func RequestURL(url string, filter []byte, callback string, userData interface{}, gasForResponse int) {
	oracle.Request(url, filter, callback, userData, gasForResponse)
}

// Handle is a response handler that writes response data to the storage.
func Handle(url string, data interface{}, code int, res []byte) {
	// ABORT if len(data) == 2, some tests use this feature.
	if data != nil && len(data.(string)) == 2 {
		util.Abort()
	}
	params := []interface{}{url, data, code, res}
	storage.Put(storage.GetContext(), "lastOracleResponse", std.Serialize(params))
}
