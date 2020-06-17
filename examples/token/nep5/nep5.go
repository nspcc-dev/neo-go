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

// GetSupply gets the token totalSupply value from VM storage
func (t Token) GetSupply(ctx storage.Context) interface{} {
	return storage.Get(ctx, t.CirculationKey)
}

// BalanceOf gets the token balance of a specific address
func (t Token) BalanceOf(ctx storage.Context, hodler []byte) interface{} {
	return storage.Get(ctx, hodler)
}

// Transfer token from one user to another
func (t Token) Transfer(ctx storage.Context, from []byte, to []byte, amount int) bool {
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

	amountTo := storage.Get(ctx, to).(int)
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

	amountFrom := storage.Get(ctx, from).(int)
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

// Mint initial supply of tokens.
func (t Token) Mint(ctx storage.Context, to []byte) bool {
	if !IsUsableAddress(t.Owner) {
		return false
	}
	minted := storage.Get(ctx, []byte("minted")).(bool)
	if minted {
		return false
	}

	storage.Put(ctx, to, t.TotalSupply)
	storage.Put(ctx, []byte("minted"), true)
	runtime.Notify("transfer", "", to, t.TotalSupply)
	return true
}
