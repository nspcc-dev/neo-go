/*
Package oracle provides interface to OracleContract native contract.
Oracles allow you to get external (non-blockchain) data using HTTPS or NeoFS
protocols.
*/
package oracle

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// These are potential response codes you get in your callback completing
// oracle request. Resulting data is only passed with Success code, it's
// nil otherwise.
const (
	Success              = 0x00
	ProtocolNotSupported = 0x10
	ConsensusUnreachable = 0x12
	NotFound             = 0x14
	Timeout              = 0x16
	Forbidden            = 0x18
	ResponseTooLarge     = 0x1a
	InsufficientFunds    = 0x1c
	Error                = 0xff
)

// Hash represents Oracle contract hash.
const Hash = "\x58\x87\x17\x11\x7e\x0a\xa8\x10\x72\xaf\xab\x71\xd2\xdd\x89\xfe\x7c\x4b\x92\xfe"

// MinimumResponseGas is the minimum response fee permitted for request (that is
// you can't attach less than that to your request). It's 0.1 GAS at the moment.
const MinimumResponseGas = 10_000_000

// Request makes an oracle request. It can only be successfully invoked by
// deployed contract and it takes the following parameters:
//
//   url
//       URL to fetch, only https and neofs URLs are supported like
//       https://example.com/some.json or
//       neofs:6pJtLUnGqDxE2EitZYLsDzsfTDVegD6BrRUn8QAFZWyt/5Cyxb3wrHDw5pqY63hb5otCSsJ24ZfYmsA8NAjtho2gr
//
//   filter
//       JSONPath filter to process the result, if specified it will be
//       applied to the data returned from HTTP/NeoFS and you'll only get
//       filtered data in your callback method.
//
//   cb
//       name of the method that will process oracle data, it must be a method
//       of the same contract that invokes Request and it must have the following
//       signature for correct invocation:
//
//           Method(url string, userData interface{}, code int, result []byte)
//
//       where url is the same url specified for Request, userData is anything
//       passed in the next parameter, code is the status of the reply and
//       result is data returned from request if any.
//
//   userData
//       data to pass to the callback function.
//
//   gasForResponse
//       GAS attached to this request for reply callback processing,
//       note that it's different from the oracle request price, this
//       GAS is used for oracle transaction's network and system fees,
//       so it should be enough to pay for reply data as well as
//       its processing.
func Request(url string, filter []byte, cb string, userData interface{}, gasForResponse int) {
	contract.Call(interop.Hash160(Hash), "request",
		contract.States|contract.AllowNotify,
		url, filter, cb, userData, gasForResponse)
}

// GetPrice returns current oracle request price.
func GetPrice() int {
	return contract.Call(interop.Hash160(Hash), "getPrice", contract.ReadStates).(int)
}

// SetPrice allows to set oracle request price. This method can only be
// successfully invoked by the committee.
func SetPrice(amount int) {
	contract.Call(interop.Hash160(Hash), "setPrice", contract.States, amount)
}
