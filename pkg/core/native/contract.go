package native

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/pkg/errors"
)

// Contracts is a set of registered native contracts.
type Contracts struct {
	NEO       *NEO
	GAS       *GAS
	Contracts []interop.Contract
}

// ByHash returns native contract with the specified hash.
func (cs *Contracts) ByHash(h util.Uint160) interop.Contract {
	for _, ctr := range cs.Contracts {
		if ctr.Metadata().Hash.Equals(h) {
			return ctr
		}
	}
	return nil
}

// ByID returns native contract with the specified id.
func (cs *Contracts) ByID(id uint32) interop.Contract {
	for _, ctr := range cs.Contracts {
		if ctr.Metadata().ServiceID == id {
			return ctr
		}
	}
	return nil
}

// NewContracts returns new set of native contracts with new GAS and NEO
// contracts.
func NewContracts() *Contracts {
	cs := new(Contracts)

	gas := NewGAS()
	neo := NewNEO()
	neo.GAS = gas
	gas.NEO = neo

	cs.GAS = gas
	cs.Contracts = append(cs.Contracts, gas)
	cs.NEO = neo
	cs.Contracts = append(cs.Contracts, neo)
	return cs
}

// GetNativeInterop returns an interop getter for a given set of contracts.
func (cs *Contracts) GetNativeInterop(ic *interop.Context) func(uint32) *vm.InteropFuncPrice {
	return func(id uint32) *vm.InteropFuncPrice {
		if c := cs.ByID(id); c != nil {
			return &vm.InteropFuncPrice{
				Func:  getNativeInterop(ic, c),
				Price: 0, // TODO price func
			}
		}
		return nil
	}
}

// getNativeInterop returns native contract interop.
func getNativeInterop(ic *interop.Context, c interop.Contract) func(v *vm.VM) error {
	return func(v *vm.VM) error {
		h := v.GetContextScriptHash(0)
		if !h.Equals(c.Metadata().Hash) {
			return errors.New("invalid hash")
		}
		name := string(v.Estack().Pop().Bytes())
		args := v.Estack().Pop().Array()
		m, ok := c.Metadata().Methods[name]
		if !ok {
			return fmt.Errorf("method %s not found", name)
		}
		result := m.Func(ic, args)
		v.Estack().PushVal(result)
		return nil
	}
}
