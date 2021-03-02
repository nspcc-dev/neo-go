package notary

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Notary contract hash.
const Hash = "\x3b\xec\x35\x31\x11\x9b\xba\xd7\x6d\xd0\x44\x92\x0b\x0d\xe6\xc3\x19\x4f\xe1\xc1"

// LockDepositUntil represents `lockDepositUntil` method of Notary native contract.
func LockDepositUntil(addr interop.Hash160, till int) bool {
	return contract.Call(interop.Hash160(Hash), "lockDepositUntil", contract.States,
		addr, till).(bool)
}

// Withdraw represents `withdraw` method of Notary native contract.
func Withdraw(from, to interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "withdraw", contract.States,
		from, to).(bool)
}

// BalanceOf represents `balanceOf` method of Notary native contract.
func BalanceOf(addr interop.Hash160) int {
	return contract.Call(interop.Hash160(Hash), "balanceOf", contract.ReadStates, addr).(int)
}

// ExpirationOf represents `expirationOf` method of Notary native contract.
func ExpirationOf(addr interop.Hash160) int {
	return contract.Call(interop.Hash160(Hash), "expirationOf", contract.ReadStates, addr).(int)
}

// GetMaxNotValidBeforeDelta represents `getMaxNotValidBeforeDelta` method of Notary native contract.
func GetMaxNotValidBeforeDelta() int {
	return contract.Call(interop.Hash160(Hash), "getMaxNotValidBeforeDelta", contract.ReadStates).(int)
}

// SetMaxNotValidBeforeDelta represents `setMaxNotValidBeforeDelta` method of Notary native contract.
func SetMaxNotValidBeforeDelta(value int) {
	contract.Call(interop.Hash160(Hash), "setMaxNotValidBeforeDelta", contract.States, value)
}
