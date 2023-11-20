/*
Package policy provides an interface to PolicyContract native contract.
This contract holds various network-wide settings.
*/
package policy

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Hash represents Policy contract hash.
const Hash = "\x7b\xc6\x81\xc0\xa1\xf7\x1d\x54\x34\x57\xb6\x8b\xba\x8d\x5f\x9f\xdd\x4e\x5e\xcc"

// GetFeePerByte represents `getFeePerByte` method of Policy native contract.
func GetFeePerByte() int {
	return neogointernal.CallWithToken(Hash, "getFeePerByte", int(contract.ReadStates)).(int)
}

// SetFeePerByte represents `setFeePerByte` method of Policy native contract.
func SetFeePerByte(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setFeePerByte", int(contract.States), value)
}

// GetExecFeeFactor represents `getExecFeeFactor` method of Policy native contract.
func GetExecFeeFactor() int {
	return neogointernal.CallWithToken(Hash, "getExecFeeFactor", int(contract.ReadStates)).(int)
}

// SetExecFeeFactor represents `setExecFeeFactor` method of Policy native contract.
func SetExecFeeFactor(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setExecFeeFactor", int(contract.States), value)
}

// GetStoragePrice represents `getStoragePrice` method of Policy native contract.
func GetStoragePrice() int {
	return neogointernal.CallWithToken(Hash, "getStoragePrice", int(contract.ReadStates)).(int)
}

// SetStoragePrice represents `setStoragePrice` method of Policy native contract.
func SetStoragePrice(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setStoragePrice", int(contract.States), value)
}

// IsBlocked represents `isBlocked` method of Policy native contract.
func IsBlocked(addr interop.Hash160) bool {
	return neogointernal.CallWithToken(Hash, "isBlocked", int(contract.ReadStates), addr).(bool)
}

// BlockAccount represents `blockAccount` method of Policy native contract.
func BlockAccount(addr interop.Hash160) bool {
	return neogointernal.CallWithToken(Hash, "blockAccount", int(contract.States), addr).(bool)
}

// UnblockAccount represents `unblockAccount` method of Policy native contract.
func UnblockAccount(addr interop.Hash160) bool {
	return neogointernal.CallWithToken(Hash, "unblockAccount", int(contract.States), addr).(bool)
}

// GetSystemFeeRefundableCost represents `getSystemFeeRefundCost` method of Policy native contract.
func GetSystemFeeRefundableCost() int {
	return neogointernal.CallWithToken(Hash, "getSystemFeeRefundCost", int(contract.ReadStates)).(int)
}

// SetSystemFeeRefundableCost represents `setSystemFeeRefundCost` method of Policy native contract.
func SetSystemFeeRefundableCost(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setSystemFeeRefundCost", int(contract.States), value)
}
