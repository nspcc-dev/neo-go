package native

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
)

// Management is contract-managing native contract.
type Management struct {
	interop.ContractMD
}

const (
	managementName        = "Neo Contract Management"
	prefixContract        = 8
	prefixNextAvailableId = 15
)

// newManagement creates new Management native contract.
func newManagement() *Management {
	var m = &Management{ContractMD: *interop.NewContractMD(managementName)}

	return m
}

// Metadata implements Contract interface.
func (m *Management) Metadata() *interop.ContractMD {
	return &m.ContractMD
}

// OnPersist implements Contract interface.
func (m *Management) OnPersist(ic *interop.Context) error {
	if ic.Block.Index != 0 { // We're only deploying at 0 at the moment.
		return nil
	}

	for _, native := range ic.Natives {
		md := native.Metadata()

		cs := &state.Contract{
			ID:       md.ContractID,
			Hash:     md.Hash,
			Script:   md.Script,
			Manifest: md.Manifest,
		}
		if err := ic.DAO.PutContractState(cs); err != nil {
			return err
		}
		if err := native.Initialize(ic); err != nil {
			return fmt.Errorf("initializing %s native contract: %w", md.Name, err)
		}
	}

	return nil
}

// PostPersist implements Contract interface.
func (m *Management) PostPersist(_ *interop.Context) error {
	return nil
}

// Initialize implements Contract interface.
func (m *Management) Initialize(_ *interop.Context) error {
	return nil
}
