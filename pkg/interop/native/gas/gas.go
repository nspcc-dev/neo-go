/*
Package gas provides interface to GasToken native contract.
It implements regular NEP-17 functions for GAS token.
*/
package gas

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Hash represents GAS contract hash.
const Hash = "\xcf\x76\xe2\x8b\xd0\x06\x2c\x4a\x47\x8e\xe3\x55\x61\x01\x13\x19\xf3\xcf\xa4\xd2"

// Symbol represents `symbol` method of GAS native contract.
func Symbol() string {
	return neogointernal.CallWithToken(Hash, "symbol", int(contract.NoneFlag)).(string)
}

// Decimals represents `decimals` method of GAS native contract.
func Decimals() int {
	return neogointernal.CallWithToken(Hash, "decimals", int(contract.NoneFlag)).(int)
}

// TotalSupply represents `totalSupply` method of GAS native contract.
func TotalSupply() int {
	return neogointernal.CallWithToken(Hash, "totalSupply", int(contract.ReadStates)).(int)
}

// BalanceOf represents `balanceOf` method of GAS native contract.
func BalanceOf(addr interop.Hash160) int {
	return neogointernal.CallWithToken(Hash, "balanceOf", int(contract.ReadStates), addr).(int)
}

// Transfer represents `transfer` method of GAS native contract.
func Transfer(from, to interop.Hash160, amount int, data interface{}) bool {
	return neogointernal.CallWithToken(Hash, "transfer",
		int(contract.All), from, to, amount, data).(bool)
}
