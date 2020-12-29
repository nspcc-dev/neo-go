package contract

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Call calls a contract.
func Call(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	method := ic.VM.Estack().Pop().String()
	args := ic.VM.Estack().Pop().Array()
	return callExInternal(ic, h, method, args, callflag.All)
}

// CallEx calls a contract with flags.
func CallEx(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	method := ic.VM.Estack().Pop().String()
	args := ic.VM.Estack().Pop().Array()
	fs := callflag.CallFlag(int32(ic.VM.Estack().Pop().BigInt().Int64()))
	if fs&^callflag.All != 0 {
		return errors.New("call flags out of range")
	}
	return callExInternal(ic, h, method, args, fs)
}

func callExInternal(ic *interop.Context, h []byte, name string, args []stackitem.Item, f callflag.CallFlag) error {
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
	return CallExInternal(ic, cs, name, args, f, vm.EnsureNotEmpty)
}

// CallExInternal calls a contract with flags and can't be invoked directly by user.
func CallExInternal(ic *interop.Context, cs *state.Contract,
	name string, args []stackitem.Item, f callflag.CallFlag, checkReturn vm.CheckReturnState) error {
	return callExFromNative(ic, ic.VM.GetCurrentScriptHash(), cs, name, args, f, checkReturn)
}

// callExFromNative calls a contract with flags using provided calling hash.
func callExFromNative(ic *interop.Context, caller util.Uint160, cs *state.Contract,
	name string, args []stackitem.Item, f callflag.CallFlag, checkReturn vm.CheckReturnState) error {
	md := cs.Manifest.ABI.GetMethod(name)
	if md == nil {
		return fmt.Errorf("method '%s' not found", name)
	}

	if len(args) != len(md.Parameters) {
		return fmt.Errorf("invalid argument count: %d (expected %d)", len(args), len(md.Parameters))
	}

	ic.VM.Invocations[cs.Hash]++
	ic.VM.LoadScriptWithCallingHash(caller, cs.NEF.Script, cs.Hash, ic.VM.Context().GetCallFlags()&f)
	var isNative bool
	for i := range ic.Natives {
		if ic.Natives[i].Metadata().Hash.Equals(cs.Hash) {
			isNative = true
			break
		}
	}
	if isNative {
		ic.VM.Estack().PushVal(args)
		ic.VM.Estack().PushVal(name)
	} else {
		for i := len(args) - 1; i >= 0; i-- {
			ic.VM.Estack().PushVal(args[i])
		}
		// use Jump not Call here because context was loaded in LoadScript above.
		ic.VM.Jump(ic.VM.Context(), md.Offset)
	}
	ic.VM.Context().CheckReturn = checkReturn

	md = cs.Manifest.ABI.GetMethod(manifest.MethodInit)
	if md != nil {
		ic.VM.Call(ic.VM.Context(), md.Offset)
	}

	return nil
}

// ErrNativeCall is returned for failed calls from native.
var ErrNativeCall = errors.New("error during call from native")

// CallFromNative performs synchronous call from native contract.
func CallFromNative(ic *interop.Context, caller util.Uint160, cs *state.Contract, method string, args []stackitem.Item, checkReturn vm.CheckReturnState) error {
	startSize := ic.VM.Istack().Len()
	if err := callExFromNative(ic, caller, cs, method, args, callflag.All, checkReturn); err != nil {
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
