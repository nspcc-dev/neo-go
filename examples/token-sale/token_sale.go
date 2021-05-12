package tokensale

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

const (
	decimals   = 8
	multiplier = decimals * 10
)

var (
	owner   = util.FromAddress("NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB")
	trigger byte
	token   TokenConfig
	ctx     storage.Context
)

// TokenConfig holds information about the token we want to use for the sale.
type TokenConfig struct {
	// Name of the token.
	Name string
	// 3 letter abbreviation of the token.
	Symbol string
	// How decimals this token will have.
	Decimals int
	// Address of the token owner. This is the Uint160 hash.
	Owner []byte
	// The total amount of tokens created. Notice that we need to multiply the
	// amount by 100000000. (10^8)
	TotalSupply int
	// Initial amount is number of tokens that are available for the token sale.
	InitialAmount int
	// How many NEO will be worth 1 token. For example:
	// Lets say 1 euro per token, where 1 NEO is 60 euro. This means buyers
	// will get (60 * 10^8) tokens for 1 NEO.
	AmountPerNEO int
	// How many Gas will be worth 1 token. This is the same calculation as
	// for the AmountPerNEO, except Gas price will have a different value.
	AmountPerGas int
	// The maximum amount you can mint in the limited round. For example:
	// 500 NEO/buyer * 60 tokens/NEO * 10^8
	MaxExchangeLimitRound int
	// When to start the token sale.
	SaleStart int
	// When to end the initial limited round if there is one. For example:
	// SaleStart + 10000
	LimitRoundEnd int
	// The prefix used to store how many tokens there are in circulation.
	CirculationKey []byte
	// The prefix used to store how many tokens there are in the limited round.
	LimitRoundKey []byte
	// The prefix used to store the addresses that are registered with KYC.
	KYCKey []byte
}

// newTokenConfig returns the initialized TokenConfig.
func newTokenConfig() TokenConfig {
	return TokenConfig{
		Name:                  "My awesome token",
		Symbol:                "MAT",
		Decimals:              decimals,
		Owner:                 owner,
		TotalSupply:           10000000 * multiplier,
		InitialAmount:         5000000 * multiplier,
		AmountPerNEO:          60 * multiplier,
		AmountPerGas:          40 * multiplier,
		MaxExchangeLimitRound: 500 * 60 * multiplier,
		SaleStart:             75500,
		LimitRoundEnd:         75500 + 10000,
		CirculationKey:        []byte("in_circulation"),
		LimitRoundKey:         []byte("r1"),
		KYCKey:                []byte("kyc_ok"),
	}
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

// InCirculation returns the amount of total tokens that are in circulation.
func InCirculation() int {
	return getIntFromDB(ctx, token.CirculationKey)
}

// addToCirculation sets the given amount as "in circulation" in the storage.
func addToCirculation(amount int) bool {
	if amount < 0 {
		return false
	}
	supply := getIntFromDB(ctx, token.CirculationKey)
	supply += amount
	if supply > token.TotalSupply {
		return false
	}
	storage.Put(ctx, token.CirculationKey, supply)
	return true
}

// AvailableAmount returns the total amount of available tokens left
// to be distributed.
func AvailableAmount() int {
	inCirc := getIntFromDB(ctx, token.CirculationKey)
	return token.TotalSupply - inCirc
}

// init initializes runtime trigger, TokenConfig and storage context before any
// other contract method is called
func init() {
	trigger = runtime.GetTrigger()
	token = newTokenConfig()
	ctx = storage.GetContext()
}

// checkOwnerWitness is a helper function which checks whether the invoker is the
// owner of the contract.
func checkOwnerWitness() bool {
	// This is used to verify if a transfer of system assets (NEO and Gas)
	// involving this contract's address can proceed.
	if trigger == runtime.Application {
		// Check if the invoker is the owner of the contract.
		return runtime.CheckWitness(token.Owner)
	}
	return false
}

// Decimals returns the token decimals
func Decimals() int {
	if trigger != runtime.Application {
		panic("invalid trigger")
	}
	return token.Decimals
}

// Symbol returns the token symbol
func Symbol() string {
	if trigger != runtime.Application {
		panic("invalid trigger")
	}
	return token.Symbol
}

// TotalSupply returns the token total supply value
func TotalSupply() int {
	if trigger != runtime.Application {
		panic("invalid trigger")
	}
	return getIntFromDB(ctx, token.CirculationKey)
}

// BalanceOf returns the amount of token on the specified address
func BalanceOf(holder interop.Hash160) int {
	if trigger != runtime.Application {
		panic("invalid trigger")
	}
	return getIntFromDB(ctx, holder)
}

// Transfer transfers specified amount of token from one user to another
func Transfer(from, to interop.Hash160, amount int, _ interface{}) bool {
	if trigger != runtime.Application {
		return false
	}
	if amount <= 0 || len(to) != 20 || !runtime.CheckWitness(from) {
		return false
	}
	amountFrom := getIntFromDB(ctx, from)
	if amountFrom < amount {
		return false
	}
	if amountFrom == amount {
		storage.Delete(ctx, from)
	} else {
		diff := amountFrom - amount
		storage.Put(ctx, from, diff)
	}
	amountTo := getIntFromDB(ctx, to)
	totalAmountTo := amountTo + amount
	storage.Put(ctx, to, totalAmountTo)
	return true
}

// TransferFrom transfers specified amount of token from one user to another.
// It differs from Transfer in that it use allowance value to store the amount
// of token available to transfer.
func TransferFrom(from, to []byte, amount int) bool {
	if trigger != runtime.Application {
		return false
	}
	if amount <= 0 {
		return false
	}
	availableKey := append(from, to...)
	if len(availableKey) != 40 {
		return false
	}
	availableTo := getIntFromDB(ctx, availableKey)
	if availableTo < amount {
		return false
	}
	fromBalance := getIntFromDB(ctx, from)
	if fromBalance < amount {
		return false
	}
	toBalance := getIntFromDB(ctx, to)
	newFromBalance := fromBalance - amount
	newToBalance := toBalance + amount
	storage.Put(ctx, to, newToBalance)
	storage.Put(ctx, from, newFromBalance)

	newAllowance := availableTo - amount
	if newAllowance == 0 {
		storage.Delete(ctx, availableKey)
	} else {
		storage.Put(ctx, availableKey, newAllowance)
	}
	return true
}

// Approve stores token transfer data if the owner has enough token to send.
func Approve(owner, spender []byte, amount int) bool {
	if !checkOwnerWitness() || amount < 0 {
		return false
	}
	if len(spender) != 20 {
		return false
	}
	toSpend := getIntFromDB(ctx, owner)
	if toSpend < amount {
		return false
	}
	approvalKey := append(owner, spender...)
	if amount == 0 {
		storage.Delete(ctx, approvalKey)
	} else {
		storage.Put(ctx, approvalKey, amount)
	}
	return true
}

// Allowance returns allowance value for specified sender and receiver.
func Allowance(from, to []byte) interface{} {
	if trigger != runtime.Application {
		return false
	}
	key := append(from, to...)
	return getIntFromDB(ctx, key)
}

// Mint initial supply of tokens
func Mint(to []byte) bool {
	if trigger != runtime.Application {
		return false
	}
	if !checkOwnerWitness() {
		return false
	}
	minted := storage.Get(ctx, []byte("minted"))
	if minted != nil && minted.(bool) == true {
		return false
	}

	storage.Put(ctx, to, token.TotalSupply)
	storage.Put(ctx, []byte("minted"), true)
	addToCirculation(token.TotalSupply)
	return true
}
