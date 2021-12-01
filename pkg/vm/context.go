package vm

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Context represents the current execution context of the VM.
type Context struct {
	// Instruction pointer.
	ip int

	// The next instruction pointer.
	nextip int

	// The raw program script.
	prog []byte

	// Breakpoints.
	breakPoints []int

	// Evaluation stack pointer.
	estack *Stack

	static    *Slot
	local     *Slot
	arguments *Slot

	// Exception context stack.
	tryStack Stack

	// Script hash of the prog.
	scriptHash util.Uint160

	// Caller's contract script hash.
	callingScriptHash util.Uint160

	// Call flags this context was created with.
	callFlag callflag.CallFlag

	// retCount specifies number of return values.
	retCount int
	// NEF represents NEF file for the current contract.
	NEF *nef.File
	// invTree is an invocation tree (or branch of it) for this context.
	invTree *InvocationTree
}

var errNoInstParam = errors.New("failed to read instruction parameter")

// NewContext returns a new Context object.
func NewContext(b []byte) *Context {
	return NewContextWithParams(b, -1, 0)
}

// NewContextWithParams creates new Context objects using script, parameter count,
// return value count and initial position in script.
func NewContextWithParams(b []byte, rvcount int, pos int) *Context {
	return &Context{
		prog:     b,
		retCount: rvcount,
		nextip:   pos,
	}
}

// Estack returns the evaluation stack of c.
func (c *Context) Estack() *Stack {
	return c.estack
}

// NextIP returns next instruction pointer.
func (c *Context) NextIP() int {
	return c.nextip
}

// Jump unconditionally moves the next instruction pointer to specified location.
func (c *Context) Jump(pos int) {
	c.nextip = pos
}

// Next returns the next instruction to execute with its parameter if any.
// The parameter is not copied and shouldn't be written to. After its invocation
// the instruction pointer points to the instruction being returned.
func (c *Context) Next() (opcode.Opcode, []byte, error) {
	var err error

	c.ip = c.nextip
	if c.ip >= len(c.prog) {
		return opcode.RET, nil, nil
	}

	var instrbyte = c.prog[c.ip]
	instr := opcode.Opcode(instrbyte)
	if !opcode.IsValid(instr) {
		return instr, nil, fmt.Errorf("incorrect opcode %s", instr.String())
	}
	c.nextip++

	var numtoread int
	switch instr {
	case opcode.PUSHDATA1:
		if c.nextip >= len(c.prog) {
			err = errNoInstParam
		} else {
			numtoread = int(c.prog[c.nextip])
			c.nextip++
		}
	case opcode.PUSHDATA2:
		if c.nextip+1 >= len(c.prog) {
			err = errNoInstParam
		} else {
			numtoread = int(binary.LittleEndian.Uint16(c.prog[c.nextip : c.nextip+2]))
			c.nextip += 2
		}
	case opcode.PUSHDATA4:
		if c.nextip+3 >= len(c.prog) {
			err = errNoInstParam
		} else {
			var n = binary.LittleEndian.Uint32(c.prog[c.nextip : c.nextip+4])
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
	if c.nextip+numtoread-1 >= len(c.prog) {
		err = errNoInstParam
	}
	if err != nil {
		return instr, nil, err
	}
	parameter := c.prog[c.nextip : c.nextip+numtoread]
	c.nextip += numtoread
	return instr, parameter, nil
}

// IP returns current instruction offset in the context script.
func (c *Context) IP() int {
	return c.ip
}

// LenInstr returns the number of instructions loaded.
func (c *Context) LenInstr() int {
	return len(c.prog)
}

// CurrInstr returns the current instruction and opcode.
func (c *Context) CurrInstr() (int, opcode.Opcode) {
	return c.ip, opcode.Opcode(c.prog[c.ip])
}

// NextInstr returns the next instruction and opcode.
func (c *Context) NextInstr() (int, opcode.Opcode) {
	op := opcode.RET
	if c.nextip < len(c.prog) {
		op = opcode.Opcode(c.prog[c.nextip])
	}
	return c.nextip, op
}

// Copy returns an new exact copy of c.
func (c *Context) Copy() *Context {
	ctx := new(Context)
	*ctx = *c
	return ctx
}

// GetCallFlags returns calling flags context was created with.
func (c *Context) GetCallFlags() callflag.CallFlag {
	return c.callFlag
}

// Program returns the loaded program.
func (c *Context) Program() []byte {
	return c.prog
}

// ScriptHash returns a hash of the script in the current context.
func (c *Context) ScriptHash() util.Uint160 {
	if c.scriptHash.Equals(util.Uint160{}) {
		c.scriptHash = hash.Hash160(c.prog)
	}
	return c.scriptHash
}

// Value implements stackitem.Item interface.
func (c *Context) Value() interface{} {
	return c
}

// Dup implements stackitem.Item interface.
func (c *Context) Dup() stackitem.Item {
	return c
}

// TryBool implements stackitem.Item interface.
func (c *Context) TryBool() (bool, error) { panic("can't convert Context to Bool") }

// TryBytes implements stackitem.Item interface.
func (c *Context) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Context to ByteArray")
}

// TryInteger implements stackitem.Item interface.
func (c *Context) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Context to Integer")
}

// Type implements stackitem.Item interface.
func (c *Context) Type() stackitem.Type { panic("Context cannot appear on evaluation stack") }

// Convert implements stackitem.Item interface.
func (c *Context) Convert(_ stackitem.Type) (stackitem.Item, error) {
	panic("Context cannot be converted to anything")
}

// Equals implements stackitem.Item interface.
func (c *Context) Equals(s stackitem.Item) bool {
	return c == s
}

func (c *Context) atBreakPoint() bool {
	for _, n := range c.breakPoints {
		if n == c.nextip {
			return true
		}
	}
	return false
}

func (c *Context) String() string {
	return "execution context"
}

// IsDeployed returns whether this context contains deployed contract.
func (c *Context) IsDeployed() bool {
	return c.NEF != nil
}

// DumpStaticSlot returns json formatted representation of the given slot.
func (c *Context) DumpStaticSlot() string {
	return dumpSlot(c.static)
}

// DumpLocalSlot returns json formatted representation of the given slot.
func (c *Context) DumpLocalSlot() string {
	return dumpSlot(c.local)
}

// DumpArgumentsSlot returns json formatted representation of the given slot.
func (c *Context) DumpArgumentsSlot() string {
	return dumpSlot(c.arguments)
}

// dumpSlot returns json formatted representation of the given slot.
func dumpSlot(s *Slot) string {
	b, _ := json.MarshalIndent(s, "", "    ")
	return string(b)
}

// getContextScriptHash returns script hash of the invocation stack element
// number n.
func (v *VM) getContextScriptHash(n int) util.Uint160 {
	istack := v.Istack()
	if istack.Len() <= n {
		return util.Uint160{}
	}
	element := istack.Peek(n)
	ctx := element.value.(*Context)
	return ctx.ScriptHash()
}

// PushContextScriptHash pushes to evaluation stack the script hash of the
// invocation stack element number n.
func (v *VM) PushContextScriptHash(n int) error {
	h := v.getContextScriptHash(n)
	v.Estack().PushItem(stackitem.NewByteArray(h.BytesBE()))
	return nil
}
