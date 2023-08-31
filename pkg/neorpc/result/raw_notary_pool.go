package result

import (
	"encoding/json"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// RawNotaryPool represents a result of `getrawnotarypool` RPC call.
// The structure consist of `Hashes`. `Hashes` field is a map, where key is
// the hash of the main transaction and value is a slice of related fallback
// transaction hashes.
type RawNotaryPool struct {
	Hashes map[util.Uint256][]util.Uint256
}

// rawNotaryPoolAux is an auxiliary struct for RawNotaryPool JSON marshalling.
type rawNotaryPoolAux struct {
	Hashes map[string][]util.Uint256 `json:"hashes,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface.
func (p RawNotaryPool) MarshalJSON() ([]byte, error) {
	var aux rawNotaryPoolAux
	aux.Hashes = make(map[string][]util.Uint256, len(p.Hashes))
	for main, fallbacks := range p.Hashes {
		aux.Hashes["0x"+main.StringLE()] = fallbacks
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (p *RawNotaryPool) UnmarshalJSON(data []byte) error {
	var aux rawNotaryPoolAux
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	p.Hashes = make(map[util.Uint256][]util.Uint256, len(aux.Hashes))
	for main, fallbacks := range aux.Hashes {
		hashMain, err := util.Uint256DecodeStringLE(strings.TrimPrefix(main, "0x"))
		if err != nil {
			return err
		}
		p.Hashes[hashMain] = fallbacks
	}
	return nil
}
