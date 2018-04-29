package tokensale

import "github.com/CityOfZion/neo-go/pkg/vm/api/storage"

const (
	decimals   = 8
	multiplier = decimals * 10
)

var owner = []byte{0xaf, 0x12, 0xa8, 0x68, 0x7b, 0x14, 0x94, 0x8b, 0xc4, 0xa0, 0x08, 0x12, 0x8a, 0x55, 0x0a, 0x63, 0x69, 0x5b, 0xc1, 0xa5}

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
		CirculationKey:        []byte("inCirculation"),
		LimitRoundKey:         []byte("R1"),
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
