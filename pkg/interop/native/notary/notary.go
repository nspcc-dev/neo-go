package notary

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Notary contract hash.
const Hash = "\x0c\xcf\x26\x94\x3f\xb5\xc9\xb6\x05\xe2\x06\xd2\xa2\x75\xbe\x3e\xa6\xa4\x75\xf4"

// LockDepositUntil represents `lockDepositUntil` method of Notary native contract.
func LockDepositUntil(addr interop.Hash160, till int) bool {
	return contract.Call(interop.Hash160(Hash), "lockDepositUntil", contract.WriteStates,
		addr, till).(bool)
}

// Withdraw represents `withdraw` method of Notary native contract.
func Withdraw(from, to interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "withdraw", contract.WriteStates,
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
	contract.Call(interop.Hash160(Hash), "setMaxNotValidBeforeDelta", contract.WriteStates, value)
}
