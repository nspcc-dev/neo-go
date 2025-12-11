package vm

import (
	"encoding/json"
	"errors"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/scparser"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/invocations"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// scriptContext is a part of the Context that is shared between multiple Contexts,
// it's created when a new script is loaded into the VM while regular
// CALL/CALLL/CALLA internal invocations reuse it.
type scriptContext struct {
	// The raw program script.
	prog []byte

	// Breakpoints.
	breakPoints []int

	// Evaluation stack pointer.
	estack *Stack

	static Slot

	// Script hash of the prog.
	scriptHash util.Uint160

	// Caller's contract script hash.
	callingScriptHash util.Uint160

	// Caller's scriptContext, if not entry.
	callingContext *scriptContext

	// Call flags this context was created with.
	callFlag callflag.CallFlag

	// NEF represents a NEF file for the current contract.
	NEF *nef.File
	// Manifest represents a manifest for the current contract.
	Manifest *manifest.Manifest
	// invTree is an invocation tree (or a branch of it) for this context.
	invTree *invocations.Tree
	// onUnload is a callback that should be called after current context unloading
	// if no exception occurs.
	onUnload ContextUnloadCallback
	// whitelisted denotes whether execution fee charging should be omitted for
	// the current context.
	whitelisted bool
}

// Context represents the current execution context of the VM.
type Context struct {
	scparser.Context

	sc *scriptContext

	local     Slot
	arguments Slot

	// Exception context stack.
	tryStack Stack

	// retCount specifies the number of return values.
	retCount int
}

type contextAux struct {
	Script string
	IP     int
	NextIP int
	Caller string
}

// ContextUnloadCallback is a callback method used on context unloading from istack.
type ContextUnloadCallback func(v *VM, ctx *Context, commit bool) error

// ErrMultiRet is returned when caller does not expect multiple return values
// from callee.
var ErrMultiRet = errors.New("multiple return values in a cross-contract call")

// NewContext returns a new Context object.
func NewContext(b []byte) *Context {
	return NewContextWithParams(b, -1, 0)
}

// NewContextWithParams creates new Context objects using script, parameter count,
// return value count and initial position in script.
func NewContextWithParams(b []byte, rvcount int, pos int) *Context {
	return &Context{
		Context: *scparser.NewContext(b, pos),
		sc: &scriptContext{
			prog: b,
		},
		retCount: rvcount,
	}
}

// Estack returns the evaluation stack of c.
func (c *Context) Estack() *Stack {
	return c.sc.estack
}

// GetCallFlags returns the calling flags which the context was created with.
func (c *Context) GetCallFlags() callflag.CallFlag {
	return c.sc.callFlag
}

// Program returns the loaded program.
func (c *Context) Program() []byte {
	return c.sc.prog
}

// ScriptHash returns a hash of the script in the current context.
func (c *Context) ScriptHash() util.Uint160 {
	if c.sc.scriptHash.Equals(util.Uint160{}) {
		c.sc.scriptHash = hash.Hash160(c.sc.prog)
	}
	return c.sc.scriptHash
}

// GetNEF returns NEF structure used by this context if it's present.
func (c *Context) GetNEF() *nef.File {
	return c.sc.NEF
}

// GetManifest returns Manifest used by this context if it's present.
func (c *Context) GetManifest() *manifest.Manifest {
	return c.sc.Manifest
}

// NumOfReturnVals returns the number of return values expected from this context.
func (c *Context) NumOfReturnVals() int {
	return c.retCount
}

func (c *Context) atBreakPoint() bool {
	return slices.Contains(c.sc.breakPoints, c.NextIP())
}

// IsDeployed returns whether this context contains a deployed contract.
func (c *Context) IsDeployed() bool {
	return c.sc.NEF != nil
}

// getContextScriptHash returns script hash of the invocation stack element
// number n.
func (v *VM) getContextScriptHash(n int) util.Uint160 {
	if len(v.istack) <= n {
		return util.Uint160{}
	}
	return v.istack[len(v.istack)-1-n].ScriptHash()
}

// IsCalledByEntry checks parent script contexts and return true if the current one
// is an entry script (the first loaded into the VM) or one called by it.
func (c *Context) IsCalledByEntry() bool {
	return c.sc.callingContext == nil || c.sc.callingContext.callingContext == nil
}

// PushContextScriptHash pushes the script hash of the
// invocation stack element number n to the evaluation stack.
func (v *VM) PushContextScriptHash(n int) error {
	h := v.getContextScriptHash(n)
	v.Estack().PushItem(stackitem.NewByteArray(h.BytesBE()))
	return nil
}

// MarshalJSON implements the JSON marshalling interface.
func (c *Context) MarshalJSON() ([]byte, error) {
	var aux = contextAux{
		Script: c.ScriptHash().StringLE(),
		IP:     c.IP(),
		NextIP: c.NextIP(),
		Caller: c.sc.callingScriptHash.StringLE(),
	}
	return json.Marshal(aux)
}

// DynamicOnUnload implements OnUnload script for dynamic calls, if no exception
// has occurred it checks that the context has exactly 0 (in which case a `Null`
// is pushed) or 1 returned value.
func DynamicOnUnload(v *VM, ctx *Context, commit bool) error {
	if commit {
		eLen := ctx.Estack().Len()
		if eLen == 0 { // No return value, add one.
			v.Context().Estack().PushItem(stackitem.Null{}) // Must use current context stack.
		} else if eLen > 1 { // Only one can be returned.
			return ErrMultiRet
		} // One value returned, it's OK.
	}
	return nil
}

// BreakPoints returns the current set of Context's breakpoints.
func (c *Context) BreakPoints() []int {
	return slices.Clone(c.sc.breakPoints)
}

// ArgumentsSlot returns the arguments slot of Context.
func (c *Context) ArgumentsSlot() *Slot {
	return &c.arguments
}

// LocalsSlot returns the local slot of Context.
func (c *Context) LocalsSlot() *Slot {
	return &c.local
}

// StaticsSlot returns the static slot of Context.
func (c *Context) StaticsSlot() *Slot {
	return &c.sc.static
}
