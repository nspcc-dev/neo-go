/*
Package notary provides an interface to Notary native contract.
This contract is a NeoGo extension and is not available on regular Neo
networks. To use it, you need to have this extension enabled on the network.
*/
package notary

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Hash represents Notary contract hash.
const Hash = "\x3b\xec\x35\x31\x11\x9b\xba\xd7\x6d\xd0\x44\x92\x0b\x0d\xe6\xc3\x19\x4f\xe1\xc1"

// LockDepositUntil represents `lockDepositUntil` method of Notary native contract.
func LockDepositUntil(addr interop.Hash160, till int) bool {
	return neogointernal.CallWithToken(Hash, "lockDepositUntil", int(contract.States),
		addr, till).(bool)
}

// Withdraw represents `withdraw` method of Notary native contract.
func Withdraw(from, to interop.Hash160) bool {
	return neogointernal.CallWithToken(Hash, "withdraw", int(contract.All),
		from, to).(bool)
}

// BalanceOf represents `balanceOf` method of Notary native contract.
func BalanceOf(addr interop.Hash160) int {
	return neogointernal.CallWithToken(Hash, "balanceOf", int(contract.ReadStates), addr).(int)
}

// ExpirationOf represents `expirationOf` method of Notary native contract.
func ExpirationOf(addr interop.Hash160) int {
	return neogointernal.CallWithToken(Hash, "expirationOf", int(contract.ReadStates), addr).(int)
}

// GetMaxNotValidBeforeDelta represents `getMaxNotValidBeforeDelta` method of Notary native contract.
func GetMaxNotValidBeforeDelta() int {
	return neogointernal.CallWithToken(Hash, "getMaxNotValidBeforeDelta", int(contract.ReadStates)).(int)
}

// SetMaxNotValidBeforeDelta represents `setMaxNotValidBeforeDelta` method of Notary native contract.
func SetMaxNotValidBeforeDelta(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setMaxNotValidBeforeDelta", int(contract.States), value)
}

// GetNotaryServiceFeePerKey represents `getNotaryServiceFeePerKey` method of Notary native contract.
func GetNotaryServiceFeePerKey() int {
	return neogointernal.CallWithToken(Hash, "getNotaryServiceFeePerKey", int(contract.ReadStates)).(int)
}

// SetNotaryServiceFeePerKey represents `setNotaryServiceFeePerKey` method of Notary native contract.
func SetNotaryServiceFeePerKey(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setNotaryServiceFeePerKey", int(contract.States), value)
}
