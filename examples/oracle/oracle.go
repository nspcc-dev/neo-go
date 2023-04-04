package oraclecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/native/oracle"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// Request does an oracle request for the URL specified. It adds minimum
// response fee which should suffice for small requests. The data from this
// URL is subsequently processed by OracleCallback function. This request
// has no JSONPath filters or user data.
func Request(url string) {
	oracle.Request(url, nil, "oracleCallback", nil, oracle.MinimumResponseGas)
}

// FilteredRequest is similar to Request but allows you to specify JSONPath filter
// to run against data got from the url specified.
func FilteredRequest(url string, filter []byte) {
	oracle.Request(url, filter, "oracleCallback", nil, oracle.MinimumResponseGas)
}

// OracleCallback is called by Oracle native contract when request is finished.
// It either throws an error (if the result is not successful) or logs the data
// got as a result.
func OracleCallback(url string, data any, code int, res []byte) {
	// This function shouldn't be called directly, we only expect oracle native
	// contract to be calling it.
	callingHash := runtime.GetCallingScriptHash()
	if !callingHash.Equals(oracle.Hash) {
		panic("not called from oracle contract")
	}
	if code != oracle.Success {
		panic("request failed for " + url + " with code " + std.Itoa(code, 10))
	}
	runtime.Log("result for " + url + ": " + string(res))
}
