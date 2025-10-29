package contract

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// LoadToken calls method specified by the token id.
func LoadToken(ic *interop.Context, id int32) error {
	ctx := ic.VM.Context()
	if !ctx.GetCallFlags().Has(callflag.ReadStates | callflag.AllowCall) {
		return errors.New("invalid call flags")
	}
	tok := ctx.GetNEF().Tokens[id]
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
	return callInternal(ic, cs, tok.Method, tok.CallFlag, tok.HasReturn, args, false)
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

	if ic.SaveInvocations {
		var (
			arrCount = len(args)
			argBytes []byte
		)
		if argBytes, err = ic.DAO.GetItemCtx().Serialize(stackitem.NewArray(args), false); err != nil {
			argBytes = nil
		}
		ci := state.NewContractInvocation(u, method, bytes.Clone(argBytes), uint32(arrCount))
		ic.InvocationCalls = append(ic.InvocationCalls, *ci)
	}
	return callInternal(ic, cs, method, fs, hasReturn, args, true)
}

func callInternal(ic *interop.Context, cs *state.Contract, name string, f callflag.CallFlag,
	hasReturn bool, args []stackitem.Item, isDynamic bool) error {
	md := cs.Manifest.ABI.GetMethod(name, len(args))
	if md.Safe {
		f &^= (callflag.WriteStates | callflag.AllowNotify)
	} else if ctx := ic.VM.Context(); ctx != nil && ctx.IsDeployed() {
		var mfst *manifest.Manifest
		if ic.IsHardforkEnabled(config.HFDomovoi) {
			mfst = ctx.GetManifest()
		} else {
			curr, err := ic.GetContract(ic.VM.GetCurrentScriptHash())
			if err == nil {
				mfst = &curr.Manifest
			}
		}
		if mfst != nil && !mfst.CanCall(cs.Hash, &cs.Manifest, name) {
			return errors.New("disallowed method call")
		}
	}
	return callExFromNative(ic, ic.VM.GetCurrentScriptHash(), cs, name, args, f, hasReturn, isDynamic, false)
}

// callExFromNative calls a contract with flags using the provided calling hash.
func callExFromNative(ic *interop.Context, caller util.Uint160, cs *state.Contract,
	name string, args []stackitem.Item, f callflag.CallFlag, hasReturn bool, isDynamic bool, callFromNative bool) error {
	var whitelisted bool
	if ic.PolicyChecker != nil {
		if ic.PolicyChecker.IsBlocked(ic.DAO, cs.Hash) {
			return fmt.Errorf("contract %s is blocked", cs.Hash.StringLE())
		}
		if ic.IsHardforkEnabled(config.HFFaun) {
			fee := ic.PolicyChecker.IsWhitelisted(ic.DAO, cs.Hash, name, len(args))
			if fee >= 0 {
				if !ic.VM.AddGas(fee) {
					return fmt.Errorf("%w during whitelisted contract execution", storage.ErrGasLimitExceeded)
				}
				whitelisted = true
			}
		}
	}
	md := cs.Manifest.ABI.GetMethod(name, len(args))
	if md == nil {
		return fmt.Errorf("method '%s' not found", name)
	}

	if len(args) != len(md.Parameters) {
		return fmt.Errorf("invalid argument count: %d (expected %d)", len(args), len(md.Parameters))
	}

	methodOff := md.Offset
	initOff := -1
	md = cs.Manifest.ABI.GetMethod(manifest.MethodInit, 0)
	if md != nil {
		initOff = md.Offset
	}
	ic.Invocations[cs.Hash]++
	f = ic.VM.Context().GetCallFlags() & f

	wrapped := ic.VM.ContractHasTryBlock() && // If the method is not wrapped into try-catch block, then changes should be discarded anyway if exception occurs.
		f&(callflag.All^callflag.ReadOnly) != 0 // If the method is safe, then it's read-only and doesn't perform storage changes or emit notifications.
	baseNtfCount := len(ic.Notifications)
	baseDAO := ic.DAO
	if wrapped {
		ic.DAO = ic.DAO.GetPrivate()
	}
	onUnload := func(v *vm.VM, ctx *vm.Context, commit bool) error {
		if wrapped {
			if commit {
				_, err := ic.DAO.Persist()
				if err != nil {
					return fmt.Errorf("failed to persist changes %w", err)
				}
			} else {
				ic.Notifications = ic.Notifications[:baseNtfCount] // Rollback all notification changes made by current context.
			}
			ic.DAO = baseDAO
		}
		if callFromNative && !commit {
			return fmt.Errorf("unhandled exception")
		}
		if isDynamic {
			return vm.DynamicOnUnload(v, ctx, commit)
		}
		return nil
	}
	ic.VM.LoadNEFMethod(&cs.NEF, &cs.Manifest, caller, cs.Hash, f,
		hasReturn, methodOff, initOff, onUnload, whitelisted)

	for e, i := ic.VM.Estack(), len(args)-1; i >= 0; i-- {
		e.PushItem(args[i])
	}
	return nil
}

// ErrNativeCall is returned for failed calls from native.
var ErrNativeCall = errors.New("failed native call")

// CallFromNative performs synchronous call from native contract.
func CallFromNative(ic *interop.Context, caller util.Uint160, cs *state.Contract, method string, args []stackitem.Item, hasReturn bool) error {
	startSize := len(ic.VM.Istack())
	if err := callExFromNative(ic, caller, cs, method, args, callflag.All, hasReturn, false, true); err != nil {
		return err
	}

	for !ic.VM.HasStopped() && len(ic.VM.Istack()) > startSize {
		if err := ic.VM.Step(); err != nil {
			return fmt.Errorf("%w: %w", ErrNativeCall, err)
		}
	}
	if ic.VM.HasFailed() {
		return ErrNativeCall
	}
	return nil
}

// GetCallFlags returns current context calling flags.
func GetCallFlags(ic *interop.Context) error {
	ic.VM.Estack().PushItem(stackitem.NewBigInteger(big.NewInt(int64(ic.VM.Context().GetCallFlags()))))
	return nil
}
