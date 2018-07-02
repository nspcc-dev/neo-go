package tokensale

import (
	"github.com/CityOfZion/neo-go/pkg/vm/api/runtime"
	"github.com/CityOfZion/neo-go/pkg/vm/api/storage"
	"github.com/CityOfZion/neo-go/pkg/vm/api/util"
)

const (
	decimals   = 8
	multiplier = decimals * 10
)

var owner = util.FromAddress("AJX1jGfj3qPBbpAKjY527nPbnrnvSx9nCg")

// TokenConfig holds information about the token we want to use for the sale.
type TokenConfig struct {
	// Name of the token.
	Name string
	// 3 letter abreviation of the token.
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

// NewTokenConfig returns the initialized TokenConfig.
func NewTokenConfig() TokenConfig {
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

// InCirculation return the amount of total tokens that are in circulation.
func (t TokenConfig) InCirculation(ctx storage.Context) int {
	amount := storage.Get(ctx, t.CirculationKey)
	return amount.(int)
}

// AddToCirculation sets the given amount as "in circulation" in the storage.
func (t TokenConfig) AddToCirculation(ctx storage.Context, amount int) bool {
	supply := storage.Get(ctx, t.CirculationKey).(int)
	supply += amount
	storage.Put(ctx, t.CirculationKey, supply)
	return true
}

// TokenSaleAvailableAmount returns the total amount of available tokens left
// to be distributed.
func (t TokenConfig) TokenSaleAvailableAmount(ctx storage.Context) int {
	inCirc := storage.Get(ctx, t.CirculationKey)
	return t.TotalSupply - inCirc.(int)
}

// Main smart contract entry point.
func Main(operation string, args []interface{}) interface{} {
	var (
		trigger = runtime.GetTrigger()
		cfg     = NewTokenConfig()
		ctx     = storage.GetContext()
	)

	// This is used to verify if a transfer of system assets (NEO and Gas)
	// involving this contract's address can proceed.
	if trigger == runtime.Verification() {
		// Check if the invoker is the owner of the contract.
		if runtime.CheckWitness(cfg.Owner) {
			return true
		}
		// Otherwise TODO
		return false
	}
	if trigger == runtime.Application() {
		return handleOperation(operation, args, ctx, cfg)
	}
	return true
}

func handleOperation(op string, args []interface{}, ctx storage.Context, cfg TokenConfig) interface{} {
	// NEP-5 handlers
	if op == "name" {
		return cfg.Name
	}
	if op == "decimals" {
		return cfg.Decimals
	}
	if op == "symbol" {
		return cfg.Symbol
	}
	if op == "totalSupply" {
		return storage.Get(ctx, cfg.CirculationKey)
	}
	if op == "balanceOf" {
		if len(args) == 1 {
			return storage.Get(ctx, args[0].([]byte))
		}
	}
	if op == "transfer" {
		if len(args) != 3 {
			return false
		}
		from := args[0].([]byte)
		to := args[1].([]byte)
		amount := args[2].(int)
		return transfer(cfg, ctx, from, to, amount)
	}
	if op == "transferFrom" {
		if len(args) != 3 {
			return false
		}
		from := args[0].([]byte)
		to := args[1].([]byte)
		amount := args[2].(int)
		return transferFrom(cfg, ctx, from, to, amount)
	}
	if op == "approve" {
		if len(args) != 3 {
			return false
		}
		from := args[0].([]byte)
		to := args[1].([]byte)
		amount := args[2].(int)
		return approve(ctx, from, to, amount)
	}
	if op == "allowance" {
		if len(args) != 2 {
			return false
		}
		from := args[0].([]byte)
		to := args[1].([]byte)
		return allowance(ctx, from, to)
	}
	return false
}

func transfer(cfg TokenConfig, ctx storage.Context, from, to []byte, amount int) bool {
	if amount <= 0 || len(to) != 20 || !runtime.CheckWitness(from) {
		return false
	}
	amountFrom := storage.Get(ctx, from).(int)
	if amountFrom < amount {
		return false
	}
	if amountFrom == amount {
		storage.Delete(ctx, from)
	} else {
		diff := amountFrom - amount
		storage.Put(ctx, from, diff)
	}
	amountTo := storage.Get(ctx, to).(int)
	totalAmountTo := amountTo + amount
	storage.Put(ctx, to, totalAmountTo)
	return true
}

func transferFrom(cfg TokenConfig, ctx storage.Context, from, to []byte, amount int) bool {
	if amount <= 0 {
		return false
	}
	availableKey := append(from, to...)
	if len(availableKey) != 40 {
		return false
	}
	availableTo := storage.Get(ctx, availableKey).(int)
	if availableTo < amount {
		return false
	}
	fromBalance := storage.Get(ctx, from).(int)
	if fromBalance < amount {
		return false
	}
	toBalance := storage.Get(ctx, to).(int)
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

func approve(ctx storage.Context, owner, spender []byte, amount int) bool {
	if !runtime.CheckWitness(owner) || amount < 0 {
		return false
	}
	toSpend := storage.Get(ctx, owner).(int)
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

func allowance(ctx storage.Context, from, to []byte) int {
	key := append(from, to...)
	return storage.Get(ctx, key).(int)
}
