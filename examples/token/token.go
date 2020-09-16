package tokencontract

import (
	"github.com/nspcc-dev/neo-go/examples/token/nep5"
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

const (
	decimals   = 8
	multiplier = 100000000
)

var (
	owner = util.FromAddress("NULwe3UAHckN2fzNdcVg31tDiaYtMDwANt")
	token nep5.Token
	ctx   storage.Context
)

// init initializes the Token Interface and storage context for the Smart
// Contract to operate with
func init() {
	token = nep5.Token{
		Name:           "Awesome NEO Token",
		Symbol:         "ANT",
		Decimals:       decimals,
		Owner:          owner,
		TotalSupply:    11000000 * multiplier,
		CirculationKey: "TokenCirculation",
	}
	ctx = storage.GetContext()
}

// Name returns the token name
func Name() string {
	return token.Name
}

// Symbol returns the token symbol
func Symbol() string {
	return token.Symbol
}

// Decimals returns the token decimals
func Decimals() int {
	return token.Decimals
}

// TotalSupply returns the token total supply value
func TotalSupply() int {
	return token.GetSupply(ctx)
}

// BalanceOf returns the amount of token on the specified address
func BalanceOf(holder interop.Hash160) interface{} {
	return token.BalanceOf(ctx, holder)
}

// Transfer token from one user to another
func Transfer(from interop.Hash160, to interop.Hash160, amount int) bool {
	return token.Transfer(ctx, from, to, amount)
}

// Mint initial supply of tokens
func Mint(to []byte) bool {
	return token.Mint(ctx, to)
}
