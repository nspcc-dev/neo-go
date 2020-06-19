package native

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/pkg/errors"
)

// Contracts is a set of registered native contracts.
type Contracts struct {
	NEO       *NEO
	GAS       *GAS
	Contracts []interop.Contract
	// persistScript is vm script which executes "onPersist" method of every native contract.
	persistScript []byte
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

// GetPersistScript returns VM script calling "onPersist" method of every native contract.
func (cs *Contracts) GetPersistScript() []byte {
	if cs.persistScript != nil {
		return cs.persistScript
	}
	w := io.NewBufBinWriter()
	for i := range cs.Contracts {
		md := cs.Contracts[i].Metadata()
		emit.Int(w.BinWriter, 0)
		emit.Opcode(w.BinWriter, opcode.NEWARRAY)
		emit.String(w.BinWriter, "onPersist")
		emit.AppCall(w.BinWriter, md.Hash)
	}
	cs.persistScript = w.Bytes()
	return cs.persistScript
}

// GetNativeInterop returns an interop getter for a given set of contracts.
func (cs *Contracts) GetNativeInterop(ic *interop.Context) func(uint32) *vm.InteropFuncPrice {
	return func(id uint32) *vm.InteropFuncPrice {
		if c := cs.ByID(id); c != nil {
			return &vm.InteropFuncPrice{
				Func: getNativeInterop(ic, c),
			}
		}
		return nil
	}
}

// getNativeInterop returns native contract interop.
func getNativeInterop(ic *interop.Context, c interop.Contract) func(v *vm.VM) error {
	return func(v *vm.VM) error {
		h := v.GetCurrentScriptHash()
		if !h.Equals(c.Metadata().Hash) {
			return errors.New("invalid hash")
		}
		name := string(v.Estack().Pop().Bytes())
		args := v.Estack().Pop().Array()
		m, ok := c.Metadata().Methods[name]
		if !ok {
			return fmt.Errorf("method %s not found", name)
		}
		if !v.Context().GetCallFlags().Has(m.RequiredFlags) {
			return errors.New("missing call flags")
		}
		if !v.AddGas(util.Fixed8(m.Price)) {
			return errors.New("gas limit exceeded")
		}
		result := m.Func(ic, args)
		v.Estack().PushVal(result)
		return nil
	}
}
