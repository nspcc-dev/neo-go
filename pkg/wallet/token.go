package wallet

import (
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Token represents imported token contract.
type Token struct {
	Name     string       `json:"name"`
	Hash     util.Uint160 `json:"script_hash"`
	Decimals int64        `json:"decimals"`
	Symbol   string       `json:"symbol"`
}

// NewToken returns new token contract info.
func NewToken(tokenHash util.Uint160, name, symbol string, decimals int64) *Token {
	return &Token{
		Name:     name,
		Hash:     tokenHash,
		Decimals: decimals,
		Symbol:   symbol,
	}
}

// Address returns token address from hash
func (t *Token) Address() string {
	return address.Uint160ToString(t.Hash)
}
