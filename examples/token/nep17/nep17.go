package nep17

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// Token holds all token info
type Token struct {
	// Token name
	Name string
	// Ticker symbol
	Symbol string
	// Amount of decimals
	Decimals int
	// Token owner address
	Owner []byte
	// Total tokens * multiplier
	TotalSupply int
	// Storage key for circulation value
	CirculationKey string
}

// getIntFromDB is a helper that checks for nil result of storage.Get and returns
// zero as the default value.
func getIntFromDB(ctx storage.Context, key []byte) int {
	var res int
	val := storage.Get(ctx, key)
	if val != nil {
		res = val.(int)
	}
	return res
}

// GetSupply gets the token totalSupply value from VM storage
func (t Token) GetSupply(ctx storage.Context) int {
	return getIntFromDB(ctx, []byte(t.CirculationKey))
}

// BalanceOf gets the token balance of a specific address
func (t Token) BalanceOf(ctx storage.Context, holder []byte) int {
	return getIntFromDB(ctx, holder)
}

// Transfer token from one user to another
func (t Token) Transfer(ctx storage.Context, from, to interop.Hash160, amount int, data interface{}) bool {
	amountFrom := t.CanTransfer(ctx, from, to, amount)
	if amountFrom == -1 {
		return false
	}

	if amountFrom == 0 {
		storage.Delete(ctx, from)
	}

	if amountFrom > 0 {
		diff := amountFrom - amount
		storage.Put(ctx, from, diff)
	}

	amountTo := getIntFromDB(ctx, to)
	totalAmountTo := amountTo + amount
	if totalAmountTo != 0 {
		storage.Put(ctx, to, totalAmountTo)
	}

	runtime.Notify("Transfer", from, to, amount)
	if to != nil && management.GetContract(to) != nil {
		contract.Call(to, "onNEP17Payment", contract.All, from, amount, data)
	}
	return true
}

// CanTransfer returns the amount it can transfer
func (t Token) CanTransfer(ctx storage.Context, from []byte, to []byte, amount int) int {
	if len(to) != 20 || !IsUsableAddress(from) {
		return -1
	}

	amountFrom := getIntFromDB(ctx, from)
	if amountFrom < amount {
		return -1
	}

	// Tell Transfer the result is equal - special case since it uses Delete
	if amountFrom == amount {
		return 0
	}

	// return amountFrom value back to Transfer, reduces extra Get
	return amountFrom
}

// IsUsableAddress checks if the sender is either the correct NEO address or SC address
func IsUsableAddress(addr []byte) bool {
	if len(addr) == 20 {

		if runtime.CheckWitness(addr) {
			return true
		}

		// Check if a smart contract is calling scripthash
		callingScriptHash := runtime.GetCallingScriptHash()
		if callingScriptHash.Equals(addr) {
			return true
		}
	}

	return false
}

// Mint initial supply of tokens
func (t Token) Mint(ctx storage.Context, to interop.Hash160) bool {
	if !IsUsableAddress(t.Owner) {
		return false
	}
	minted := storage.Get(ctx, []byte("minted"))
	if minted != nil && minted.(bool) == true {
		return false
	}

	storage.Put(ctx, to, t.TotalSupply)
	storage.Put(ctx, []byte("minted"), true)
	storage.Put(ctx, []byte(t.CirculationKey), t.TotalSupply)
	var from interop.Hash160
	runtime.Notify("Transfer", from, to, t.TotalSupply)
	return true
}
