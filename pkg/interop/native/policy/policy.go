package policy

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Policy contract hash.
const Hash = "\x7b\xc6\x81\xc0\xa1\xf7\x1d\x54\x34\x57\xb6\x8b\xba\x8d\x5f\x9f\xdd\x4e\x5e\xcc"

// GetMaxBlockSize represents `getMaxBlockSize` method of Policy native contract.
func GetMaxBlockSize() int {
	return contract.Call(interop.Hash160(Hash), "getMaxBlockSize", contract.ReadStates).(int)
}

// SetMaxBlockSize represents `setMaxBlockSize` method of Policy native contract.
func SetMaxBlockSize(value int) {
	contract.Call(interop.Hash160(Hash), "setMaxBlockSize", contract.States, value)
}

// GetFeePerByte represents `getFeePerByte` method of Policy native contract.
func GetFeePerByte() int {
	return contract.Call(interop.Hash160(Hash), "getFeePerByte", contract.ReadStates).(int)
}

// SetFeePerByte represents `setFeePerByte` method of Policy native contract.
func SetFeePerByte(value int) {
	contract.Call(interop.Hash160(Hash), "setFeePerByte", contract.States, value)
}

// GetMaxBlockSystemFee represents `getMaxBlockSystemFee` method of Policy native contract.
func GetMaxBlockSystemFee() int {
	return contract.Call(interop.Hash160(Hash), "getMaxBlockSystemFee", contract.ReadStates).(int)
}

// SetMaxBlockSystemFee represents `setMaxBlockSystemFee` method of Policy native contract.
func SetMaxBlockSystemFee(value int) {
	contract.Call(interop.Hash160(Hash), "setMaxBlockSystemFee", contract.States, value)
}

// GetExecFeeFactor represents `getExecFeeFactor` method of Policy native contract.
func GetExecFeeFactor() int {
	return contract.Call(interop.Hash160(Hash), "getExecFeeFactor", contract.ReadStates).(int)
}

// SetExecFeeFactor represents `setExecFeeFactor` method of Policy native contract.
func SetExecFeeFactor(value int) {
	contract.Call(interop.Hash160(Hash), "setExecFeeFactor", contract.States, value)
}

// GetStoragePrice represents `getStoragePrice` method of Policy native contract.
func GetStoragePrice() int {
	return contract.Call(interop.Hash160(Hash), "getStoragePrice", contract.ReadStates).(int)
}

// SetStoragePrice represents `setStoragePrice` method of Policy native contract.
func SetStoragePrice(value int) {
	contract.Call(interop.Hash160(Hash), "setStoragePrice", contract.States, value)
}

// IsBlocked represents `isBlocked` method of Policy native contract.
func IsBlocked(addr interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "isBlocked", contract.ReadStates, addr).(bool)
}

// BlockAccount represents `blockAccount` method of Policy native contract.
func BlockAccount(addr interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "blockAccount", contract.States, addr).(bool)
}

// UnblockAccount represents `unblockAccount` method of Policy native contract.
func UnblockAccount(addr interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "unblockAccount", contract.States, addr).(bool)
}
