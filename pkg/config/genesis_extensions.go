package config

import (
	"encoding/base64"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
)

// Genesis represents a set of genesis block settings including the extensions
// enabled in the genesis block or during native contracts initialization.
type Genesis struct {
	// MaxValidUntilBlockIncrement is the upper increment size of blockchain
	// height (in blocks) exceeding that a transaction should fail validation.
	// It differs from Protocol level configuration in that this value is used
	// starting from HFEchidna to initialize MaxValidUntilBlockIncrement value
	// of native Policy contract.
	MaxValidUntilBlockIncrement uint32
	// Roles contains the set of roles that should be designated during native
	// Designation contract initialization. It is NeoGo extension and must be
	// disabled on the public Neo N3 networks.
	Roles map[noderoles.Role]keys.PublicKeys
	// Transaction contains transaction script that should be deployed in the
	// genesis block. It is NeoGo extension and must be disabled on the public
	// Neo N3 networks.
	Transaction *GenesisTransaction
}

// GenesisTransaction is a placeholder for script that should be included into genesis
// block as a transaction script with the given system fee. Provided
// system fee value will be taken from the standby validators account which is
// added to the list of Signers as a sender with CalledByEntry scope.
type GenesisTransaction struct {
	Script    []byte
	SystemFee int64
}

type (
	// genesisAux is an auxiliary structure for Genesis YAML marshalling.
	genesisAux struct {
		MaxValidUntilBlockIncrement uint32                     `yaml:"MaxValidUntilBlockIncrement"`
		Roles                       map[string]keys.PublicKeys `yaml:"Roles"`
		Transaction                 *genesisTransactionAux     `yaml:"Transaction"`
	}
	// genesisTransactionAux is an auxiliary structure for GenesisTransaction YAML
	// marshalling.
	genesisTransactionAux struct {
		Script    string `yaml:"Script"`
		SystemFee int64  `yaml:"SystemFee"`
	}
)

// MarshalYAML implements the YAML marshaler interface.
func (e Genesis) MarshalYAML() (any, error) {
	var aux genesisAux
	aux.Roles = make(map[string]keys.PublicKeys, len(e.Roles))
	for r, ks := range e.Roles {
		aux.Roles[r.String()] = ks
	}
	if e.Transaction != nil {
		aux.Transaction = &genesisTransactionAux{
			Script:    base64.StdEncoding.EncodeToString(e.Transaction.Script),
			SystemFee: e.Transaction.SystemFee,
		}
	}
	aux.MaxValidUntilBlockIncrement = e.MaxValidUntilBlockIncrement
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

	if aux.Transaction != nil {
		script, err := base64.StdEncoding.DecodeString(aux.Transaction.Script)
		if err != nil {
			return fmt.Errorf("failed to decode script of genesis transaction: %w", err)
		}
		e.Transaction = &GenesisTransaction{
			Script:    script,
			SystemFee: aux.Transaction.SystemFee,
		}
	}

	e.MaxValidUntilBlockIncrement = aux.MaxValidUntilBlockIncrement

	return nil
}
