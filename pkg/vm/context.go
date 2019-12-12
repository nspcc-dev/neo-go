package vm

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
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

	// Return value count, -1 is unspecified.
	rvcount int

	// Evaluation stack pointer.
	estack *Stack

	// Alt stack pointer.
	astack *Stack
}

// NewContext returns a new Context object.
func NewContext(b []byte) *Context {
	return &Context{
		prog:        b,
		breakPoints: []int{},
		rvcount:     -1,
	}
}

// Next returns the next instruction to execute with its parameter if any. After
// its invocation the instruction pointer points to the instruction being
// returned.
func (c *Context) Next() (opcode.Opcode, []byte, error) {
	c.ip = c.nextip
	if c.ip >= len(c.prog) {
		return opcode.RET, nil, nil
	}
	r := io.NewBinReaderFromBuf(c.prog[c.ip:])

	var instrbyte = r.ReadByte()
	instr := opcode.Opcode(instrbyte)
	c.nextip++

	var numtoread int
	switch instr {
	case opcode.PUSHDATA1, opcode.SYSCALL:
		var n = r.ReadByte()
		numtoread = int(n)
		c.nextip++
	case opcode.PUSHDATA2:
		var n = r.ReadU16LE()
		numtoread = int(n)
		c.nextip += 2
	case opcode.PUSHDATA4:
		var n = r.ReadU32LE()
		if n > MaxItemSize {
			return instr, nil, errors.New("parameter is too big")
		}
		numtoread = int(n)
		c.nextip += 4
	case opcode.JMP, opcode.JMPIF, opcode.JMPIFNOT, opcode.CALL, opcode.CALLED, opcode.CALLEDT:
		numtoread = 2
	case opcode.CALLI:
		numtoread = 4
	case opcode.APPCALL, opcode.TAILCALL:
		numtoread = 20
	case opcode.CALLE, opcode.CALLET:
		numtoread = 22
	default:
		if instr >= opcode.PUSHBYTES1 && instr <= opcode.PUSHBYTES75 {
			numtoread = int(instr)
		} else {
			// No parameters, can just return.
			return instr, nil, nil
		}
	}
	parameter := make([]byte, numtoread)
	r.ReadBytes(parameter)
	if r.Err != nil {
		return instr, nil, errors.New("failed to read instruction parameter")
	}
	c.nextip += numtoread
	return instr, parameter, nil
}

// IP returns the absolute instruction without taking 0 into account.
// If that program starts the ip = 0 but IP() will return 1, cause its
// the first instruction.
func (c *Context) IP() int {
	return c.ip + 1
}

// LenInstr returns the number of instructions loaded.
func (c *Context) LenInstr() int {
	return len(c.prog)
}

// CurrInstr returns the current instruction and opcode.
func (c *Context) CurrInstr() (int, opcode.Opcode) {
	return c.ip, opcode.Opcode(c.prog[c.ip])
}

// Copy returns an new exact copy of c.
func (c *Context) Copy() *Context {
	ctx := new(Context)
	*ctx = *c
	return ctx
}

// Program returns the loaded program.
func (c *Context) Program() []byte {
	return c.prog
}

// Value implements StackItem interface.
func (c *Context) Value() interface{} {
	return c
}

func (c *Context) atBreakPoint() bool {
	for _, n := range c.breakPoints {
		if n == c.ip {
			return true
		}
	}
	return false
}

func (c *Context) String() string {
	return "execution context"
}
