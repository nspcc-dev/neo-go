package callback

import (
	"errors"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// MethodCallback represents callback for contract method.
type MethodCallback struct {
	contract *state.Contract
	method   *manifest.Method
}

var _ Callback = (*MethodCallback)(nil)

// ArgCount implements Callback interface.
func (s *MethodCallback) ArgCount() int {
	return len(s.method.Parameters)
}

// LoadContext implements Callback interface.
func (s *MethodCallback) LoadContext(v *vm.VM, args []stackitem.Item) {
	v.Estack().PushVal(args)
	v.Estack().PushVal(s.method.Name)
	v.Estack().PushVal(s.contract.Hash.BytesBE())
}

// CreateFromMethod creates callback for a contract method.
func CreateFromMethod(ic *interop.Context) error {
	rawHash := ic.VM.Estack().Pop().Bytes()
	h, err := util.Uint160DecodeBytesBE(rawHash)
	if err != nil {
		return err
	}
	cs, err := ic.DAO.GetContractState(h)
	if err != nil {
		return errors.New("contract not found")
	}
	method := string(ic.VM.Estack().Pop().Bytes())
	if strings.HasPrefix(method, "_") {
		return errors.New("invalid method name")
	}
	currCs, err := ic.DAO.GetContractState(ic.VM.GetCurrentScriptHash())
	if err == nil && !currCs.Manifest.CanCall(h, &cs.Manifest, method) {
		return errors.New("method call is not allowed")
	}
	md := cs.Manifest.ABI.GetMethod(method)
	ic.VM.Estack().PushVal(stackitem.NewInterop(&MethodCallback{
		contract: cs,
		method:   md,
	}))
	return nil
}
