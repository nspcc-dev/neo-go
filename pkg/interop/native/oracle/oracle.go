package oracle

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Oracle contract hash.
const Hash = "\x58\x87\x17\x11\x7e\x0a\xa8\x10\x72\xaf\xab\x71\xd2\xdd\x89\xfe\x7c\x4b\x92\xfe"

// Request represents `request` method of Oracle native contract.
func Request(url string, filter []byte, cb string, userData interface{}, gasForResponse int) {
	contract.Call(interop.Hash160(Hash), "request",
		contract.States|contract.AllowNotify,
		url, filter, cb, userData, gasForResponse)
}

// GetPrice represents `getPrice` method of Oracle native contract.
func GetPrice() int {
	return contract.Call(interop.Hash160(Hash), "getPrice", contract.ReadStates).(int)
}

// SetPrice represents `setPrice` method of Oracle native contract.
func SetPrice(amount int) {
	contract.Call(interop.Hash160(Hash), "setPrice", contract.States, amount)
}
