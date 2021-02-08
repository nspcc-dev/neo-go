package policy

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Policy contract hash.
const Hash = "\xf2\xe2\x08\xed\xcd\x14\x6c\xbe\xe4\x67\x6e\xdf\x79\xb7\x5e\x50\x98\xd3\xbc\x79"

// GetMaxTransactionsPerBlock represents `getMaxTransactionsPerBlock` method of Policy native contract.
func GetMaxTransactionsPerBlock() int {
	return contract.Call(interop.Hash160(Hash), "getMaxTransactionsPerBlock", contract.ReadStates).(int)
}

// SetMaxTransactionsPerBlock represents `setMaxTransactionsPerBlock` method of Policy native contract.
func SetMaxTransactionsPerBlock(value int) {
	contract.Call(interop.Hash160(Hash), "setMaxTransactionsPerBlock", contract.WriteStates, value)
}

// GetMaxBlockSize represents `getMaxBlockSize` method of Policy native contract.
func GetMaxBlockSize() int {
	return contract.Call(interop.Hash160(Hash), "getMaxBlockSize", contract.ReadStates).(int)
}

// SetMaxBlockSize represents `setMaxBlockSize` method of Policy native contract.
func SetMaxBlockSize(value int) {
	contract.Call(interop.Hash160(Hash), "setMaxBlockSize", contract.WriteStates, value)
}

// GetFeePerByte represents `getFeePerByte` method of Policy native contract.
func GetFeePerByte() int {
	return contract.Call(interop.Hash160(Hash), "getFeePerByte", contract.ReadStates).(int)
}

// SetFeePerByte represents `setFeePerByte` method of Policy native contract.
func SetFeePerByte(value int) {
	contract.Call(interop.Hash160(Hash), "setFeePerByte", contract.WriteStates, value)
}

// GetMaxBlockSystemFee represents `getMaxBlockSystemFee` method of Policy native contract.
func GetMaxBlockSystemFee() int {
	return contract.Call(interop.Hash160(Hash), "getMaxBlockSystemFee", contract.ReadStates).(int)
}

// SetMaxBlockSystemFee represents `setMaxBlockSystemFee` method of Policy native contract.
func SetMaxBlockSystemFee(value int) {
	contract.Call(interop.Hash160(Hash), "setMaxBlockSystemFee", contract.WriteStates, value)
}

// GetExecFeeFactor represents `getExecFeeFactor` method of Policy native contract.
func GetExecFeeFactor() int {
	return contract.Call(interop.Hash160(Hash), "getExecFeeFactor", contract.ReadStates).(int)
}

// SetExecFeeFactor represents `setExecFeeFactor` method of Policy native contract.
func SetExecFeeFactor(value int) {
	contract.Call(interop.Hash160(Hash), "setExecFeeFactor", contract.WriteStates, value)
}

// GetStoragePrice represents `getStoragePrice` method of Policy native contract.
func GetStoragePrice() int {
	return contract.Call(interop.Hash160(Hash), "getStoragePrice", contract.ReadStates).(int)
}

// SetStoragePrice represents `setStoragePrice` method of Policy native contract.
func SetStoragePrice(value int) {
	contract.Call(interop.Hash160(Hash), "setStoragePrice", contract.WriteStates, value)
}

// IsBlocked represents `isBlocked` method of Policy native contract.
func IsBlocked(addr interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "isBlocked", contract.ReadStates, addr).(bool)
}

// BlockAccount represents `blockAccount` method of Policy native contract.
func BlockAccount(addr interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "blockAccount", contract.WriteStates, addr).(bool)
}

// UnblockAccount represents `unblockAccount` method of Policy native contract.
func UnblockAccount(addr interop.Hash160) bool {
	return contract.Call(interop.Hash160(Hash), "unblockAccount", contract.WriteStates, addr).(bool)
}
