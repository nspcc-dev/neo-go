package nep5

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
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
func (t Token) GetSupply(ctx storage.Context) interface{} {
	return getIntFromDB(ctx, []byte(t.CirculationKey))
}

// TBalanceOf gets the token balance of a specific address
// TODO: https://github.com/nspcc-dev/neo-go/issues/1150
func (t Token) TBalanceOf(ctx storage.Context, holder []byte) interface{} {
	return getIntFromDB(ctx, holder)
}

// TTransfer token from one user to another
func (t Token) TTransfer(ctx storage.Context, from []byte, to []byte, amount int) bool {
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
	storage.Put(ctx, to, totalAmountTo)
	runtime.Notify("transfer", from, to, amount)
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
		if util.Equals(callingScriptHash, addr) {
			return true
		}
	}

	return false
}

// TMint initial supply of tokens.
func (t Token) TMint(ctx storage.Context, to []byte) bool {
	if !IsUsableAddress(t.Owner) {
		return false
	}
	minted := storage.Get(ctx, []byte("minted"))
	if minted != nil && minted.(bool) == true {
		return false
	}

	storage.Put(ctx, to, t.TotalSupply)
	storage.Put(ctx, []byte("minted"), true)
	runtime.Notify("transfer", "", to, t.TotalSupply)
	return true
}
