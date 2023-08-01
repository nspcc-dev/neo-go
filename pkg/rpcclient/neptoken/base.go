/*
Package neptoken contains RPC wrapper for common NEP-11 and NEP-17 methods.

All of these methods are safe, read-only.
*/
package neptoken

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// MaxValidDecimals is the maximum value 'decimals' contract method can
	// return to be considered as valid. It's log10(2^256), higher values
	// don't make any sense on a VM with 256-bit integers. This restriction
	// is not imposed by NEP-17 or NEP-11, but we do it as a sanity check
	// anyway (and return plain int as a result).
	MaxValidDecimals = 77
)

// Invoker is used by Base to call various methods.
type Invoker interface {
	Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error)
}

// Base is a reader interface for common NEP-11 and NEP-17 methods built
// on top of Invoker.
type Base struct {
	invoker Invoker
	hash    util.Uint160
}

// New creates an instance of Base for contract with the given hash using the
// given invoker.
func New(invoker Invoker, hash util.Uint160) *Base {
	return &Base{invoker, hash}
}

// Decimals implements `decimals` NEP-17 or NEP-11 method and returns the number
// of decimals used by token. For non-divisible NEP-11 tokens this method always
// returns zero. Values less than 0 or more than MaxValidDecimals are considered
// to be invalid (with an appropriate error) even if returned by the contract.
func (b *Base) Decimals() (int, error) {
	r, err := b.invoker.Call(b.hash, "decimals")
	dec, err := unwrap.LimitedInt64(r, err, 0, MaxValidDecimals)
	return int(dec), err
}

// Symbol implements `symbol` NEP-17 or NEP-11 method and returns a short token
// identifier (like "NEO" or "GAS").
func (b *Base) Symbol() (string, error) {
	return unwrap.PrintableASCIIString(b.invoker.Call(b.hash, "symbol"))
}

// TotalSupply returns the total token supply currently available (the amount
// of minted tokens).
func (b *Base) TotalSupply() (*big.Int, error) {
	return unwrap.BigInt(b.invoker.Call(b.hash, "totalSupply"))
}

// BalanceOf returns the token balance of the given account. For NEP-17 that's
// the token balance with decimals (1 TOK with 2 decimals will lead to 100
// returned from this method). For non-divisible NEP-11 that's the number of
// NFTs owned by the account, for divisible NEP-11 that's the sum of the parts
// of all NFTs owned by the account.
func (b *Base) BalanceOf(account util.Uint160) (*big.Int, error) {
	return unwrap.BigInt(b.invoker.Call(b.hash, "balanceOf", account))
}
