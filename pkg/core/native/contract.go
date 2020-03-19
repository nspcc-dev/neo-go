package native

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/pkg/errors"
)

// Method is a signature for a native method.
type Method = func(ic *interop.Context, args []vm.StackItem) vm.StackItem

// MethodAndPrice is a native-contract method descriptor.
type MethodAndPrice struct {
	Func          Method
	Price         int64
	RequiredFlags smartcontract.CallFlag
}

// Contract is an interface for all native contracts.
type Contract interface {
	Metadata() *ContractMD
	OnPersist(*interop.Context) error
}

// ContractMD represents native contract instance.
type ContractMD struct {
	Manifest    manifest.Manifest
	ServiceName string
	ServiceID   uint32
	Script      []byte
	Hash        util.Uint160
	Methods     map[string]MethodAndPrice
}

// Contracts is a set of registered native contracts.
type Contracts struct {
	Contracts []Contract
}

// NewContractMD returns Contract with the specified list of methods.
func NewContractMD(name string) *ContractMD {
	c := &ContractMD{
		ServiceName: name,
		ServiceID:   vm.InteropNameToID([]byte(name)),
		Methods:     make(map[string]MethodAndPrice),
	}

	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, c.ServiceName)
	c.Script = w.Bytes()
	c.Hash = hash.Hash160(c.Script)
	c.Manifest = *manifest.DefaultManifest(c.Hash)

	return c
}

// ByHash returns native contract with the specified hash.
func (cs *Contracts) ByHash(h util.Uint160) Contract {
	for _, ctr := range cs.Contracts {
		if ctr.Metadata().Hash.Equals(h) {
			return ctr
		}
	}
	return nil
}

// ByID returns native contract with the specified id.
func (cs *Contracts) ByID(id uint32) Contract {
	for _, ctr := range cs.Contracts {
		if ctr.Metadata().ServiceID == id {
			return ctr
		}
	}
	return nil
}

// AddMethod adds new method to a native contract.
func (c *ContractMD) AddMethod(md *MethodAndPrice, desc *manifest.Method, safe bool) {
	c.Manifest.ABI.Methods = append(c.Manifest.ABI.Methods, *desc)
	c.Methods[desc.Name] = *md
	if safe {
		c.Manifest.SafeMethods.Add(desc.Name)
	}
}

// AddEvent adds new event to a native contract.
func (c *ContractMD) AddEvent(name string, ps ...manifest.Parameter) {
	c.Manifest.ABI.Events = append(c.Manifest.ABI.Events, manifest.Event{
		Name:       name,
		Parameters: ps,
	})
}

// NewContracts returns new empty set of native contracts.
func NewContracts() *Contracts {
	return &Contracts{
		Contracts: []Contract{},
	}
}

// Add adds new native contracts to the list.
func (cs *Contracts) Add(c Contract) {
	cs.Contracts = append(cs.Contracts, c)
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
func getNativeInterop(ic *interop.Context, c Contract) func(v *vm.VM) error {
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
