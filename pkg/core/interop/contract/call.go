package contract

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

type policyChecker interface {
	IsBlockedInternal(dao.DAO, util.Uint160) bool
}

// LoadToken calls method specified by token id.
func LoadToken(ic *interop.Context) func(id int32) error {
	return func(id int32) error {
		ctx := ic.VM.Context()
		if !ctx.GetCallFlags().Has(callflag.ReadStates | callflag.AllowCall) {
			return errors.New("invalid call flags")
		}
		tok := ctx.NEF.Tokens[id]
		if int(tok.ParamCount) > ctx.Estack().Len() {
			return errors.New("stack is too small")
		}
		args := make([]stackitem.Item, tok.ParamCount)
		for i := range args {
			args[i] = ic.VM.Estack().Pop().Item()
		}
		cs, err := ic.GetContract(tok.Hash)
		if err != nil {
			return fmt.Errorf("token contract %s not found: %w", tok.Hash.StringLE(), err)
		}
		return callInternal(ic, cs, tok.Method, tok.CallFlag, tok.HasReturn, args)
	}
}

// Call calls a contract with flags.
func Call(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	method := ic.VM.Estack().Pop().String()
	fs := callflag.CallFlag(int32(ic.VM.Estack().Pop().BigInt().Int64()))
	if fs&^callflag.All != 0 {
		return errors.New("call flags out of range")
	}
	args := ic.VM.Estack().Pop().Array()
	u, err := util.Uint160DecodeBytesBE(h)
	if err != nil {
		return errors.New("invalid contract hash")
	}
	cs, err := ic.GetContract(u)
	if err != nil {
		return fmt.Errorf("called contract %s not found: %w", u.StringLE(), err)
	}
	if strings.HasPrefix(method, "_") {
		return errors.New("invalid method name (starts with '_')")
	}
	md := cs.Manifest.ABI.GetMethod(method, len(args))
	if md == nil {
		return fmt.Errorf("method not found: %s/%d", method, len(args))
	}
	hasReturn := md.ReturnType != smartcontract.VoidType
	if !hasReturn {
		ic.VM.Estack().PushItem(stackitem.Null{})
	}
	return callInternal(ic, cs, method, fs, hasReturn, args)
}

func callInternal(ic *interop.Context, cs *state.Contract, name string, f callflag.CallFlag,
	hasReturn bool, args []stackitem.Item) error {
	md := cs.Manifest.ABI.GetMethod(name, len(args))
	if md.Safe {
		f &^= (callflag.WriteStates | callflag.AllowNotify)
	} else if ctx := ic.VM.Context(); ctx != nil && ctx.IsDeployed() {
		curr, err := ic.GetContract(ic.VM.GetCurrentScriptHash())
		if err == nil {
			if !curr.Manifest.CanCall(cs.Hash, &cs.Manifest, name) {
				return errors.New("disallowed method call")
			}
		}
	}
	return callExFromNative(ic, ic.VM.GetCurrentScriptHash(), cs, name, args, f, hasReturn)
}

// callExFromNative calls a contract with flags using provided calling hash.
func callExFromNative(ic *interop.Context, caller util.Uint160, cs *state.Contract,
	name string, args []stackitem.Item, f callflag.CallFlag, hasReturn bool) error {
	for _, nc := range ic.Natives {
		if nc.Metadata().Name == nativenames.Policy {
			var pch = nc.(policyChecker)
			if pch.IsBlockedInternal(ic.DAO, cs.Hash) {
				return fmt.Errorf("contract %s is blocked", cs.Hash.StringLE())
			}
			break
		}
	}
	md := cs.Manifest.ABI.GetMethod(name, len(args))
	if md == nil {
		return fmt.Errorf("method '%s' not found", name)
	}

	if len(args) != len(md.Parameters) {
		return fmt.Errorf("invalid argument count: %d (expected %d)", len(args), len(md.Parameters))
	}

	ic.VM.Invocations[cs.Hash]++
	ic.VM.LoadScriptWithCallingHash(caller, cs.NEF.Script, cs.Hash, ic.VM.Context().GetCallFlags()&f, hasReturn, uint16(len(args)))
	ic.VM.Context().NEF = &cs.NEF
	for e, i := ic.VM.Estack(), len(args)-1; i >= 0; i-- {
		e.PushItem(args[i])
	}
	// Move IP to the target method.
	ic.VM.Context().Jump(md.Offset)

	md = cs.Manifest.ABI.GetMethod(manifest.MethodInit, 0)
	if md != nil {
		ic.VM.Call(ic.VM.Context(), md.Offset)
	}

	return nil
}

// ErrNativeCall is returned for failed calls from native.
var ErrNativeCall = errors.New("failed native call")

// CallFromNative performs synchronous call from native contract.
func CallFromNative(ic *interop.Context, caller util.Uint160, cs *state.Contract, method string, args []stackitem.Item, hasReturn bool) error {
	startSize := ic.VM.Istack().Len()
	if err := callExFromNative(ic, caller, cs, method, args, callflag.All, hasReturn); err != nil {
		return err
	}

	for !ic.VM.HasStopped() && ic.VM.Istack().Len() > startSize {
		if err := ic.VM.Step(); err != nil {
			return fmt.Errorf("%w: %v", ErrNativeCall, err)
		}
	}
	if ic.VM.State() == vm.FaultState {
		return ErrNativeCall
	}
	return nil
}
