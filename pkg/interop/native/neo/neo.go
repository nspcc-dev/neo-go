package neo

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents NEO contract hash.
const Hash = "\x83\xab\x06\x79\xad\x55\xc0\x50\xa1\x3a\xd4\x3f\x59\x36\xea\x73\xf5\xeb\x1e\xf6"

// Symbol represents `symbol` method of NEO native contract.
func Symbol() string {
	return contract.Call(interop.Hash160(Hash), "symbol", contract.NoneFlag).(string)
}

// Decimals represents `decimals` method of NEO native contract.
func Decimals() int {
	return contract.Call(interop.Hash160(Hash), "decimals", contract.NoneFlag).(int)
}

// TotalSupply represents `totalSupply` method of NEO native contract.
func TotalSupply() int {
	return contract.Call(interop.Hash160(Hash), "totalSupply", contract.ReadStates).(int)
}

// BalanceOf represents `balanceOf` method of NEO native contract.
func BalanceOf(addr interop.Hash160) int {
	return contract.Call(interop.Hash160(Hash), "balanceOf", contract.ReadStates, addr).(int)
}

// Transfer represents `transfer` method of NEO native contract.
func Transfer(from, to interop.Hash160, amount int, data interface{}) bool {
	return contract.Call(interop.Hash160(Hash), "transfer",
		contract.WriteStates|contract.AllowCall|contract.AllowNotify, from, to, amount, data).(bool)
}

// GetCommittee represents `getCommittee` method of NEO native contract.
func GetCommittee() []interop.PublicKey {
	return contract.Call(interop.Hash160(Hash), "getCommittee", contract.ReadStates).([]interop.PublicKey)
}

// GetCandidates represents `getCandidates` method of NEO native contract.
func GetCandidates() []interop.PublicKey {
	return contract.Call(interop.Hash160(Hash), "getCandidates", contract.ReadStates).([]interop.PublicKey)
}

// GetNextBlockValidators represents `getNextBlockValidators` method of NEO native contract.
func GetNextBlockValidators() []interop.PublicKey {
	return contract.Call(interop.Hash160(Hash), "getNextBlockValidators", contract.ReadStates).([]interop.PublicKey)
}

// GetGASPerBlock represents `getGasPerBlock` method of NEO native contract.
func GetGASPerBlock() int {
	return contract.Call(interop.Hash160(Hash), "getGasPerBlock", contract.ReadStates).(int)
}

// SetGASPerBlock represents `setGasPerBlock` method of NEO native contract.
func SetGASPerBlock(amount int) {
	contract.Call(interop.Hash160(Hash), "setGasPerBlock", contract.WriteStates, amount)
}

// RegisterCandidate represents `registerCandidate` method of NEO native contract.
func RegisterCandidate(pub interop.PublicKey) bool {
	return contract.Call(interop.Hash160(Hash), "registerCandidate", contract.WriteStates, pub).(bool)
}

// UnregisterCandidate represents `unregisterCandidate` method of NEO native contract.
func UnregisterCandidate(pub interop.PublicKey) bool {
	return contract.Call(interop.Hash160(Hash), "unregisterCandidate", contract.WriteStates, pub).(bool)
}

// Vote represents `vote` method of NEO native contract.
func Vote(addr interop.Hash160, pub interop.PublicKey) bool {
	return contract.Call(interop.Hash160(Hash), "vote", contract.WriteStates, addr, pub).(bool)
}

// UnclaimedGAS represents `unclaimedGas` method of NEO native contract.
func UnclaimedGAS(addr interop.Hash160, end int) int {
	return contract.Call(interop.Hash160(Hash), "unclaimedGas", contract.ReadStates, addr, end).(int)
}
