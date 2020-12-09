package result

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// UnclaimedGas response wrapper
type UnclaimedGas struct {
	Address   util.Uint160
	Unclaimed big.Int
}

// unclaimedGas is an auxiliary struct for JSON marhsalling
type unclaimedGas struct {
	Address   string `json:"address"`
	Unclaimed string `json:"unclaimed"`
}

// MarshalJSON implements json.Marshaler interface.
func (g UnclaimedGas) MarshalJSON() ([]byte, error) {
	gas := &unclaimedGas{
		Address:   address.Uint160ToString(g.Address),
		Unclaimed: fixedn.ToString(&g.Unclaimed, 8),
	}
	return json.Marshal(gas)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (g *UnclaimedGas) UnmarshalJSON(data []byte) error {
	gas := new(unclaimedGas)
	if err := json.Unmarshal(data, gas); err != nil {
		return err
	}
	uncl, err := fixedn.FromString(gas.Unclaimed, 8)
	if err != nil {
		return fmt.Errorf("failed to convert unclaimed gas: %w", err)
	}
	g.Unclaimed = *uncl
	addr, err := address.StringToUint160(gas.Address)
	if err != nil {
		return err
	}
	g.Address = addr
	return nil
}
