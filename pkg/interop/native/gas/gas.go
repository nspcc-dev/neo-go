package gas

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents GAS contract hash.
const Hash = "\x28\xb3\xad\xab\x72\x69\xf9\xc2\x18\x1d\xb3\xcb\x74\x1e\xbf\x55\x19\x30\xe2\x70"

// Symbol represents `symbol` method of GAS native contract.
func Symbol() string {
	return contract.Call(interop.Hash160(Hash), "symbol", contract.NoneFlag).(string)
}

// Decimals represents `decimals` method of GAS native contract.
func Decimals() int {
	return contract.Call(interop.Hash160(Hash), "decimals", contract.NoneFlag).(int)
}

// TotalSupply represents `totalSupply` method of GAS native contract.
func TotalSupply() int {
	return contract.Call(interop.Hash160(Hash), "totalSupply", contract.ReadStates).(int)
}

// BalanceOf represents `balanceOf` method of GAS native contract.
func BalanceOf(addr interop.Hash160) int {
	return contract.Call(interop.Hash160(Hash), "balanceOf", contract.ReadStates, addr).(int)
}

// Transfer represents `transfer` method of GAS native contract.
func Transfer(from, to interop.Hash160, amount int, data interface{}) bool {
	return contract.Call(interop.Hash160(Hash), "transfer",
		contract.All, from, to, amount, data).(bool)
}
