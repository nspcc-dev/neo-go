package contract

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Call calls a contract with flags.
func Call(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	method := ic.VM.Estack().Pop().String()
	fs := callflag.CallFlag(int32(ic.VM.Estack().Pop().BigInt().Int64()))
	if fs&^callflag.All != 0 {
		return errors.New("call flags out of range")
	}
	args := ic.VM.Estack().Pop().Array()
	return callInternal(ic, h, method, fs, args)
}

func callInternal(ic *interop.Context, h []byte, name string, f callflag.CallFlag, args []stackitem.Item) error {
	u, err := util.Uint160DecodeBytesBE(h)
	if err != nil {
		return errors.New("invalid contract hash")
	}
	cs, err := ic.GetContract(u)
	if err != nil {
		return fmt.Errorf("contract not found: %w", err)
	}
	if strings.HasPrefix(name, "_") {
		return errors.New("invalid method name (starts with '_')")
	}
	md := cs.Manifest.ABI.GetMethod(name)
	if md == nil {
		return errors.New("method not found")
	}
	hasReturn := md.ReturnType != smartcontract.VoidType
	if !hasReturn {
		ic.VM.Estack().PushVal(stackitem.Null{})
	}
	if md.Safe {
		f &^= callflag.WriteStates
	} else if ctx := ic.VM.Context(); ctx != nil && ctx.IsDeployed() {
		curr, err := ic.GetContract(ic.VM.GetCurrentScriptHash())
		if err == nil {
			if !curr.Manifest.CanCall(u, &cs.Manifest, name) {
				return errors.New("disallowed method call")
			}
		}
	}
	return callExFromNative(ic, ic.VM.GetCurrentScriptHash(), cs, name, args, f, hasReturn)
}

// callExFromNative calls a contract with flags using provided calling hash.
func callExFromNative(ic *interop.Context, caller util.Uint160, cs *state.Contract,
	name string, args []stackitem.Item, f callflag.CallFlag, hasReturn bool) error {
	md := cs.Manifest.ABI.GetMethod(name)
	if md == nil {
		return fmt.Errorf("method '%s' not found", name)
	}

	if len(args) != len(md.Parameters) {
		return fmt.Errorf("invalid argument count: %d (expected %d)", len(args), len(md.Parameters))
	}

	ic.VM.Invocations[cs.Hash]++
	ic.VM.LoadScriptWithCallingHash(caller, cs.NEF.Script, cs.Hash, ic.VM.Context().GetCallFlags()&f, true, uint16(len(args)))
	var isNative bool
	for i := range ic.Natives {
		if ic.Natives[i].Metadata().Hash.Equals(cs.Hash) {
			isNative = true
			break
		}
	}
	for i := len(args) - 1; i >= 0; i-- {
		ic.VM.Estack().PushVal(args[i])
	}
	if isNative {
		ic.VM.Estack().PushVal(name)
	} else {
		// use Jump not Call here because context was loaded in LoadScript above.
		ic.VM.Jump(ic.VM.Context(), md.Offset)
	}
	if hasReturn {
		ic.VM.Context().RetCount = 1
	} else {
		ic.VM.Context().RetCount = 0
	}

	md = cs.Manifest.ABI.GetMethod(manifest.MethodInit)
	if md != nil {
		ic.VM.Call(ic.VM.Context(), md.Offset)
	}

	return nil
}

// ErrNativeCall is returned for failed calls from native.
var ErrNativeCall = errors.New("error during call from native")

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
