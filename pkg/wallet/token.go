package wallet

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Token represents imported token contract.
type Token struct {
	Name     string
	Hash     util.Uint160
	Decimals int64
	Symbol   string
	Address  string
}

type tokenAux struct {
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
		Address:  address.Uint160ToString(tokenHash),
	}
}

// MarshalJSON implements json.Marshaler interface.
func (t *Token) MarshalJSON() ([]byte, error) {
	m := &tokenAux{
		Name:     t.Name,
		Hash:     t.Hash.Reverse(), // address should be marshaled in LE but default marshaler uses BE.
		Decimals: t.Decimals,
		Symbol:   t.Symbol,
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (t *Token) UnmarshalJSON(data []byte) error {
	aux := new(tokenAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	t.Name = aux.Name
	t.Hash = aux.Hash.Reverse()
	t.Decimals = aux.Decimals
	t.Symbol = aux.Symbol
	t.Address = address.Uint160ToString(t.Hash)
	return nil
}
