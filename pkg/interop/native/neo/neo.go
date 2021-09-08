/*
Package neo provides interface to NeoToken native contract.
NEO token is special, it's not just a regular NEP-17 contract, it also
provides access to chain-specific settings and implements committee
voting system.
*/
package neo

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// AccountState contains info about NEO holder.
type AccountState struct {
	Balance int
	Height  int
	VoteTo  interop.PublicKey
}

// Hash represents NEO contract hash.
const Hash = "\xf5\x63\xea\x40\xbc\x28\x3d\x4d\x0e\x05\xc4\x8e\xa3\x05\xb3\xf2\xa0\x73\x40\xef"

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
		contract.All, from, to, amount, data).(bool)
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
	contract.Call(interop.Hash160(Hash), "setGasPerBlock", contract.States, amount)
}

// GetRegisterPrice represents `getRegisterPrice` method of NEO native contract.
func GetRegisterPrice() int {
	return contract.Call(interop.Hash160(Hash), "getRegisterPrice", contract.ReadStates).(int)
}

// SetRegisterPrice represents `setRegisterPrice` method of NEO native contract.
func SetRegisterPrice(amount int) {
	contract.Call(interop.Hash160(Hash), "setRegisterPrice", contract.States, amount)
}

// RegisterCandidate represents `registerCandidate` method of NEO native contract.
func RegisterCandidate(pub interop.PublicKey) bool {
	return contract.Call(interop.Hash160(Hash), "registerCandidate", contract.States, pub).(bool)
}

// UnregisterCandidate represents `unregisterCandidate` method of NEO native contract.
func UnregisterCandidate(pub interop.PublicKey) bool {
	return contract.Call(interop.Hash160(Hash), "unregisterCandidate", contract.States, pub).(bool)
}

// Vote represents `vote` method of NEO native contract.
func Vote(addr interop.Hash160, pub interop.PublicKey) bool {
	return contract.Call(interop.Hash160(Hash), "vote", contract.States, addr, pub).(bool)
}

// UnclaimedGAS represents `unclaimedGas` method of NEO native contract.
func UnclaimedGAS(addr interop.Hash160, end int) int {
	return contract.Call(interop.Hash160(Hash), "unclaimedGas", contract.ReadStates, addr, end).(int)
}

// GetAccountState represents `getAccountState` method of NEO native contract.
func GetAccountState(addr interop.Hash160) *AccountState {
	return contract.Call(interop.Hash160(Hash), "getAccountState", contract.ReadStates, addr).(*AccountState)
}
