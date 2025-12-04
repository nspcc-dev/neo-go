package scparser

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var errNoInstParam = errors.New("failed to read instruction parameter")

// Context represents bytecode parser context.
type Context struct {
	// Instruction pointer.
	ip int
	// The next instruction pointer.
	nextip int
	// Program.
	prog []byte
}

// NewContext creates a new parsing context for the given program with the
// given initial offset.
func NewContext(b []byte, pos int) *Context {
	return &Context{
		nextip: pos,
		prog:   b,
	}
}

// NextIP returns the next instruction pointer.
func (c *Context) NextIP() int {
	return c.nextip
}

// Jump unconditionally moves the next instruction pointer to the specified location.
func (c *Context) Jump(pos int) {
	if pos < 0 || pos >= len(c.prog) {
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
	prog := c.prog
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
	return len(c.prog)
}

// CurrInstr returns the current instruction and opcode.
func (c *Context) CurrInstr() (int, opcode.Opcode) {
	return c.ip, opcode.Opcode(c.prog[c.ip])
}

// NextInstr returns the next instruction offset and opcode at that position.
// If the next instruction offset points past the end of the program the opcode
// returned is [opcode.RET].
func (c *Context) NextInstr() (int, opcode.Opcode) {
	op := opcode.RET
	if c.nextip < len(c.prog) {
		op = opcode.Opcode(c.prog[c.nextip])
	}
	return c.nextip, op
}
