package vm

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/invocations"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
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

	static slot

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
	// invTree is an invocation tree (or a branch of it) for this context.
	invTree *invocations.Tree
	// onUnload is a callback that should be called after current context unloading
	// if no exception occurs.
	onUnload ContextUnloadCallback
}

// Context represents the current execution context of the VM.
type Context struct {
	// Instruction pointer.
	ip int

	// The next instruction pointer.
	nextip int

	sc *scriptContext

	local     slot
	arguments slot

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

var errNoInstParam = errors.New("failed to read instruction parameter")

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
		sc: &scriptContext{
			prog: b,
		},
		retCount: rvcount,
		nextip:   pos,
	}
}

// Estack returns the evaluation stack of c.
func (c *Context) Estack() *Stack {
	return c.sc.estack
}

// NextIP returns the next instruction pointer.
func (c *Context) NextIP() int {
	return c.nextip
}

// Jump unconditionally moves the next instruction pointer to the specified location.
func (c *Context) Jump(pos int) {
	if pos < 0 || pos >= len(c.sc.prog) {
		panic("instruction offset is out of range")
	}
	c.nextip = pos
}

// Next returns the next instruction to execute with its parameter if any.
// The parameter is not copied and shouldn't be written to. After its invocation,
// the instruction pointer points to the instruction returned.
func (c *Context) Next() (opcode.Opcode, []byte, error) {
	var err error

	c.ip = c.nextip
	prog := c.sc.prog
	if c.ip >= len(prog) {
		return opcode.RET, nil, nil
	}

	var instrbyte = prog[c.ip]
	instr := opcode.Opcode(instrbyte)
	if !opcode.IsValid(instr) {
		return instr, nil, fmt.Errorf("incorrect opcode %s", instr.String())
	}
	c.nextip++

	var numtoread int
	switch instr {
	case opcode.PUSHDATA1:
		if c.nextip >= len(prog) {
			err = errNoInstParam
		} else {
			numtoread = int(prog[c.nextip])
			c.nextip++
		}
	case opcode.PUSHDATA2:
		if c.nextip+1 >= len(prog) {
			err = errNoInstParam
		} else {
			numtoread = int(binary.LittleEndian.Uint16(prog[c.nextip : c.nextip+2]))
			c.nextip += 2
		}
	case opcode.PUSHDATA4:
		if c.nextip+3 >= len(prog) {
			err = errNoInstParam
		} else {
			var n = binary.LittleEndian.Uint32(prog[c.nextip : c.nextip+4])
			if n > stackitem.MaxSize {
				return instr, nil, errors.New("parameter is too big")
			}
			numtoread = int(n)
			c.nextip += 4
		}
	case opcode.JMP, opcode.JMPIF, opcode.JMPIFNOT, opcode.JMPEQ, opcode.JMPNE,
		opcode.JMPGT, opcode.JMPGE, opcode.JMPLT, opcode.JMPLE,
		opcode.CALL, opcode.ISTYPE, opcode.CONVERT, opcode.NEWARRAYT,
		opcode.ENDTRY,
		opcode.INITSSLOT, opcode.LDSFLD, opcode.STSFLD, opcode.LDARG, opcode.STARG, opcode.LDLOC, opcode.STLOC:
		numtoread = 1
	case opcode.INITSLOT, opcode.TRY, opcode.CALLT:
		numtoread = 2
	case opcode.JMPL, opcode.JMPIFL, opcode.JMPIFNOTL, opcode.JMPEQL, opcode.JMPNEL,
		opcode.JMPGTL, opcode.JMPGEL, opcode.JMPLTL, opcode.JMPLEL,
		opcode.ENDTRYL,
		opcode.CALLL, opcode.SYSCALL, opcode.PUSHA:
		numtoread = 4
	case opcode.TRYL:
		numtoread = 8
	default:
		if instr <= opcode.PUSHINT256 {
			numtoread = 1 << instr
		} else {
			// No parameters, can just return.
			return instr, nil, nil
		}
	}
	if c.nextip+numtoread-1 >= len(prog) {
		err = errNoInstParam
	}
	if err != nil {
		return instr, nil, err
	}
	parameter := prog[c.nextip : c.nextip+numtoread]
	c.nextip += numtoread
	return instr, parameter, nil
}

// IP returns the current instruction offset in the context script.
func (c *Context) IP() int {
	return c.ip
}

// LenInstr returns the number of instructions loaded.
func (c *Context) LenInstr() int {
	return len(c.sc.prog)
}

// CurrInstr returns the current instruction and opcode.
func (c *Context) CurrInstr() (int, opcode.Opcode) {
	return c.ip, opcode.Opcode(c.sc.prog[c.ip])
}

// NextInstr returns the next instruction and opcode.
func (c *Context) NextInstr() (int, opcode.Opcode) {
	op := opcode.RET
	if c.nextip < len(c.sc.prog) {
		op = opcode.Opcode(c.sc.prog[c.nextip])
	}
	return c.nextip, op
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

// NumOfReturnVals returns the number of return values expected from this context.
func (c *Context) NumOfReturnVals() int {
	return c.retCount
}

func (c *Context) atBreakPoint() bool {
	for _, n := range c.sc.breakPoints {
		if n == c.nextip {
			return true
		}
	}
	return false
}

// IsDeployed returns whether this context contains a deployed contract.
func (c *Context) IsDeployed() bool {
	return c.sc.NEF != nil
}

// DumpStaticSlot returns json formatted representation of the given slot.
func (c *Context) DumpStaticSlot() string {
	return dumpSlot(&c.sc.static)
}

// DumpLocalSlot returns json formatted representation of the given slot.
func (c *Context) DumpLocalSlot() string {
	return dumpSlot(&c.local)
}

// DumpArgumentsSlot returns json formatted representation of the given slot.
func (c *Context) DumpArgumentsSlot() string {
	return dumpSlot(&c.arguments)
}

// dumpSlot returns json formatted representation of the given slot.
func dumpSlot(s *slot) string {
	if s == nil || *s == nil {
		return "[]"
	}
	b, _ := json.MarshalIndent(s, "", "    ")
	return string(b)
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
		IP:     c.ip,
		NextIP: c.nextip,
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
	res := make([]int, len(c.sc.breakPoints))
	copy(res, c.sc.breakPoints)
	return res
}
