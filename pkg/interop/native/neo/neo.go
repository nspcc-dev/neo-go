/*
Package neo provides an interface to NeoToken native contract.
NEO token is special, it's not just a regular NEP-17 contract, it also
provides access to chain-specific settings and implements committee
voting system.
*/
package neo

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// AccountState contains info about a NEO holder.
type AccountState struct {
	Balance        int
	Height         int
	VoteTo         interop.PublicKey
	LastGasPerVote int
}

// Hash represents NEO contract hash.
const Hash = "\xf5\x63\xea\x40\xbc\x28\x3d\x4d\x0e\x05\xc4\x8e\xa3\x05\xb3\xf2\xa0\x73\x40\xef"

// Symbol represents `symbol` method of NEO native contract.
func Symbol() string {
	return neogointernal.CallWithToken(Hash, "symbol", int(contract.NoneFlag)).(string)
}

// Decimals represents `decimals` method of NEO native contract.
func Decimals() int {
	return neogointernal.CallWithToken(Hash, "decimals", int(contract.NoneFlag)).(int)
}

// TotalSupply represents `totalSupply` method of NEO native contract.
func TotalSupply() int {
	return neogointernal.CallWithToken(Hash, "totalSupply", int(contract.ReadStates)).(int)
}

// BalanceOf represents `balanceOf` method of NEO native contract.
func BalanceOf(addr interop.Hash160) int {
	return neogointernal.CallWithToken(Hash, "balanceOf", int(contract.ReadStates), addr).(int)
}

// Transfer represents `transfer` method of NEO native contract.
func Transfer(from, to interop.Hash160, amount int, data any) bool {
	return neogointernal.CallWithToken(Hash, "transfer",
		int(contract.All), from, to, amount, data).(bool)
}

// GetCommittee represents `getCommittee` method of NEO native contract.
func GetCommittee() []interop.PublicKey {
	return neogointernal.CallWithToken(Hash, "getCommittee", int(contract.ReadStates)).([]interop.PublicKey)
}

// GetCandidates represents `getCandidates` method of NEO native contract. It
// returns up to 256 candidates. Use GetAllCandidates in case if you need the
// whole set of candidates.
func GetCandidates() []Candidate {
	return neogointernal.CallWithToken(Hash, "getCandidates", int(contract.ReadStates)).([]Candidate)
}

// GetAllCandidates represents `getAllCandidates` method of NEO native contract.
// It returns Iterator over the whole set of Neo candidates sorted by public key
// bytes. Each iterator value can be cast to Candidate. Use iterator interop
// package to work with the returned Iterator.
func GetAllCandidates() iterator.Iterator {
	return neogointernal.CallWithToken(Hash, "getAllCandidates", int(contract.ReadStates)).(iterator.Iterator)
}

// GetCandidateVote represents `getCandidateVote` method of NEO native contract.
// It returns -1 if the candidate hasn't been registered or voted for and the
// overall candidate votes otherwise.
func GetCandidateVote(pub interop.PublicKey) int {
	return neogointernal.CallWithToken(Hash, "getCandidateVote", int(contract.ReadStates), pub).(int)
}

// GetNextBlockValidators represents `getNextBlockValidators` method of NEO native contract.
func GetNextBlockValidators() []interop.PublicKey {
	return neogointernal.CallWithToken(Hash, "getNextBlockValidators", int(contract.ReadStates)).([]interop.PublicKey)
}

// GetGASPerBlock represents `getGasPerBlock` method of NEO native contract.
func GetGASPerBlock() int {
	return neogointernal.CallWithToken(Hash, "getGasPerBlock", int(contract.ReadStates)).(int)
}

// SetGASPerBlock represents `setGasPerBlock` method of NEO native contract.
func SetGASPerBlock(amount int) {
	neogointernal.CallWithTokenNoRet(Hash, "setGasPerBlock", int(contract.States), amount)
}

// GetRegisterPrice represents `getRegisterPrice` method of NEO native contract.
func GetRegisterPrice() int {
	return neogointernal.CallWithToken(Hash, "getRegisterPrice", int(contract.ReadStates)).(int)
}

// SetRegisterPrice represents `setRegisterPrice` method of NEO native contract.
func SetRegisterPrice(amount int) {
	neogointernal.CallWithTokenNoRet(Hash, "setRegisterPrice", int(contract.States), amount)
}

// RegisterCandidate represents `registerCandidate` method of NEO native contract.
func RegisterCandidate(pub interop.PublicKey) bool {
	return neogointernal.CallWithToken(Hash, "registerCandidate", int(contract.States), pub).(bool)
}

// UnregisterCandidate represents `unregisterCandidate` method of NEO native contract.
func UnregisterCandidate(pub interop.PublicKey) bool {
	return neogointernal.CallWithToken(Hash, "unregisterCandidate", int(contract.States), pub).(bool)
}

// Vote represents `vote` method of NEO native contract.
func Vote(addr interop.Hash160, pub interop.PublicKey) bool {
	return neogointernal.CallWithToken(Hash, "vote", int(contract.States), addr, pub).(bool)
}

// UnclaimedGAS represents `unclaimedGas` method of NEO native contract.
func UnclaimedGAS(addr interop.Hash160, end int) int {
	return neogointernal.CallWithToken(Hash, "unclaimedGas", int(contract.ReadStates), addr, end).(int)
}

// GetAccountState represents `getAccountState` method of NEO native contract.
func GetAccountState(addr interop.Hash160) *AccountState {
	return neogointernal.CallWithToken(Hash, "getAccountState", int(contract.ReadStates), addr).(*AccountState)
}

// GetCommitteeAddress represents `getCommitteeAddress` method of NEO native contract.
func GetCommitteeAddress() interop.Hash160 {
	return neogointernal.CallWithToken(Hash, "getCommitteeAddress", int(contract.ReadStates)).(interop.Hash160)

}
