package tokencontract

import (
	"github.com/nspcc-dev/neo-go/examples/token/nep5"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

const (
	decimals   = 8
	multiplier = 100000000
)

var owner = util.FromAddress("NMipL5VsNoLUBUJKPKLhxaEbPQVCZnyJyB")

// createToken initializes the Token Interface for the Smart Contract to operate with
func createToken() nep5.Token {
	return nep5.Token{
		Name:           "Awesome NEO Token",
		Symbol:         "ANT",
		Decimals:       decimals,
		Owner:          owner,
		TotalSupply:    11000000 * multiplier,
		CirculationKey: "TokenCirculation",
	}
}

// Main function = contract entry
func Main(operation string, args []interface{}) interface{} {
	if operation == "name" {
		return Name()
	}
	if operation == "symbol" {
		return Symbol()
	}
	if operation == "decimals" {
		return Decimals()
	}

	if operation == "totalSupply" {
		return TotalSupply()
	}

	if operation == "balanceOf" {
		hodler := args[0].([]byte)
		return BalanceOf(hodler)
	}

	if operation == "transfer" && checkArgs(args, 3) {
		from := args[0].([]byte)
		to := args[1].([]byte)
		amount := args[2].(int)
		return Transfer(from, to, amount)
	}

	if operation == "mint" && checkArgs(args, 1) {
		addr := args[0].([]byte)
		return Mint(addr)
	}

	return true
}

// checkArgs checks args array against a length indicator
func checkArgs(args []interface{}, length int) bool {
	if len(args) == length {
		return true
	}

	return false
}

// Name returns the token name
func Name() string {
	t := createToken()
	return t.Name
}

// Symbol returns the token symbol
func Symbol() string {
	t := createToken()
	return t.Symbol
}

// Decimals returns the token decimals
func Decimals() int {
	t := createToken()
	return t.Decimals
}

// TotalSupply returns the token total supply value
func TotalSupply() interface{} {
	t := createToken()
	ctx := storage.GetContext()
	return t.GetSupply(ctx)
}

// BalanceOf returns the amount of token on the specified address
func BalanceOf(holder []byte) interface{} {
	t := createToken()
	ctx := storage.GetContext()
	return t.TBalanceOf(ctx, holder)
}

// Transfer token from one user to another
func Transfer(from []byte, to []byte, amount int) bool {
	t := createToken()
	ctx := storage.GetContext()
	return t.TTransfer(ctx, from, to, amount)
}

// Mint initial supply of tokens
func Mint(to []byte) bool {
	t := createToken()
	ctx := storage.GetContext()
	return t.TMint(ctx, to)
}
