/*
Package policy provides an interface to PolicyContract native contract.
This contract holds various network-wide settings.
*/
package policy

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
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
// It returns the execution fee factor in Datoshi units.
func GetExecFeeFactor() int {
	return neogointernal.CallWithToken(Hash, "getExecFeeFactor", int(contract.ReadStates)).(int)
}

// GetExecPicoFeeFactor represents `getExecPicoFeeFactor` method of Policy native contract.
// It returns the execution fee factor in picoGAS units. Note that this method is available
// starting from [config.HFFaun] hardfork.
func GetExecPicoFeeFactor() int {
	return neogointernal.CallWithToken(Hash, "getExecPicoFeeFactor", int(contract.ReadStates)).(int)
}

// SetExecFeeFactor represents `setExecFeeFactor` method of Policy native contract.
// Note that starting from [config.HFFaun] hardfork this method accepts the value
// in picoGAS units instead of Datoshi units.
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

// GetAttributeFee represents `getAttributeFee` method of Policy native contract.
func GetAttributeFee(t AttributeType) int {
	return neogointernal.CallWithToken(Hash, "getAttributeFee", int(contract.ReadStates), t).(int)
}

// SetAttributeFee represents `setAttributeFee` method of Policy native contract.
func SetAttributeFee(t AttributeType, value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setAttributeFee", int(contract.States), t, value)
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

// GetMaxValidUntilBlockIncrement represents `getMaxValidUntilBlockIncrement` method of Policy native contract.
// Note that this method is available starting from [config.HFEchidna] hardfork.
func GetMaxValidUntilBlockIncrement() int {
	return neogointernal.CallWithToken(Hash, "getMaxValidUntilBlockIncrement", int(contract.ReadStates)).(int)
}

// SetMaxValidUntilBlockIncrement represents `setMaxValidUntilBlockIncrement` method of Policy native contract.
// Note that this method is available starting from [config.HFEchidna] hardfork.
func SetMaxValidUntilBlockIncrement(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setMaxValidUntilBlockIncrement", int(contract.States), value)
}

// GetMillisecondsPerBlock represents `getMillisecondsPerBlock` method of Policy native contract.
// Note that this method is available starting from [config.HFEchidna] hardfork.
func GetMillisecondsPerBlock() int {
	return neogointernal.CallWithToken(Hash, "getMillisecondsPerBlock", int(contract.ReadStates)).(int)
}

// SetMillisecondsPerBlock represents `setMaxValidUntilBlockIncrement` method of Policy native contract.
// Note that this method is available starting from [config.HFEchidna] hardfork.
func SetMillisecondsPerBlock(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setMillisecondsPerBlock", int(contract.States|contract.AllowNotify), value)
}

// GetBlockedAccounts represents `getBlockedAccounts` method of Policy native contract.
// Note that this method is available starting from [config.HFFaun] hardfork.
func GetBlockedAccounts() iterator.Iterator {
	return neogointernal.CallWithToken(Hash, "getBlockedAccounts", int(contract.ReadStates)).(iterator.Iterator)
}

// SetWhitelistFeeContract represents the `setWhitelistFeeContract` method of Policy native contract.
// Note that this method is available starting from [config.HFFaun] hardfork.
func SetWhitelistFeeContract(hash interop.Hash160, method string, argCnt int, fixedFee int) {
	neogointernal.CallWithTokenNoRet(Hash, "setWhitelistFeeContract", int(contract.States|contract.AllowNotify),
		hash, method, argCnt, fixedFee)
}

// RemoveWhitelistFeeContract represents the `removeWhitelistFeeContract` method of Policy native contract.
// Note that this method is available starting from [config.HFFaun] hardfork.
func RemoveWhitelistFeeContract(hash interop.Hash160, method string, argCnt int) {
	neogointernal.CallWithTokenNoRet(Hash, "removeWhitelistFeeContract", int(contract.States|contract.AllowNotify),
		hash, method, argCnt)
}

// GetWhitelistFeeContracts represents the `getWhitelistFeeContracts` method of Policy native contract.
// Note that this method is available starting from [config.HFFaun] hardfork.
func GetWhitelistFeeContracts() iterator.Iterator {
	return neogointernal.CallWithToken(Hash, "getWhitelistFeeContracts", int(contract.ReadStates)).(iterator.Iterator)
}
