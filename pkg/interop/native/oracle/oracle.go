package oracle

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Oracle contract hash.
const Hash = "\xee\x80\x4c\x14\x29\x68\xd4\x78\x8b\x8a\xff\x51\xda\xde\xdf\xcb\x42\xe7\xc0\x8d"

// Request represents `request` method of Oracle native contract.
func Request(url string, filter []byte, cb string, userData interface{}, gasForResponse int) {
	contract.Call(interop.Hash160(Hash), "request",
		contract.WriteStates|contract.AllowNotify,
		url, filter, cb, userData, gasForResponse)
}
