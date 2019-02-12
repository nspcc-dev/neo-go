package wrappers

import (
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// AccountState wrapper used for the representation of
// core.AccountState on the RPC Server.
type AccountState struct {
	Version    uint8                  `json:"version"`
	Address    string                 `json:"address"`
	ScriptHash util.Uint160           `json:"script_hash"`
	IsFrozen   bool                   `json:"frozen"`
	Votes      []*crypto.PublicKey    `json:"votes"`
	Balances   map[string]util.Fixed8 `json:"balances"`
}

// NewAccountState creates a new AccountState wrapper.
func NewAccountState(a *core.AccountState) AccountState {
	balances := make(map[string]util.Fixed8)
	address := crypto.AddressFromUint160(a.ScriptHash)

	for k, v := range a.Balances {
		balances[k.String()] = v
	}

	// reverse scriptHash to be consistent with other client
	scriptHash, err := util.Uint160DecodeBytes(a.ScriptHash.BytesReverse())
	if err != nil {
		scriptHash = a.ScriptHash
	}

	return AccountState{
		Version:    a.Version,
		ScriptHash: scriptHash,
		IsFrozen:   a.IsFrozen,
		Votes:      a.Votes,
		Balances:   balances,
		Address:    address,
	}
}
