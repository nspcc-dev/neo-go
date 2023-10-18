package config

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
)

// Genesis represents a set of genesis block settings including the extensions
// enabled in the genesis block or during native contracts initialization.
type Genesis struct {
	// Roles contains the set of roles that should be designated during native
	// Designation contract initialization. It is NeoGo extension and must be
	// disabled on the public Neo N3 networks.
	Roles map[noderoles.Role]keys.PublicKeys
}

// genesisAux is an auxiliary structure for Genesis YAML marshalling.
type genesisAux struct {
	Roles map[string]keys.PublicKeys `yaml:"Roles"`
}

// MarshalYAML implements the YAML marshaler interface.
func (e Genesis) MarshalYAML() (any, error) {
	var aux genesisAux
	aux.Roles = make(map[string]keys.PublicKeys, len(e.Roles))
	for r, ks := range e.Roles {
		aux.Roles[r.String()] = ks
	}
	return aux, nil
}

// UnmarshalYAML implements the YAML unmarshaler interface.
func (e *Genesis) UnmarshalYAML(unmarshal func(any) error) error {
	var aux genesisAux
	if err := unmarshal(&aux); err != nil {
		return err
	}

	e.Roles = make(map[noderoles.Role]keys.PublicKeys)
	for s, ks := range aux.Roles {
		r, ok := noderoles.FromString(s)
		if !ok {
			return fmt.Errorf("unknown node role: %s", s)
		}
		e.Roles[r] = ks
	}

	return nil
}
