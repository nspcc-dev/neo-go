package vm

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/io"
)

// Context represent the current execution context of the VM.
type Context struct {
	// Instruction pointer.
	ip int

	// The next instruction pointer.
	nextip int

	// The raw program script.
	prog []byte

	// Breakpoints
	breakPoints []int
}

// NewContext return a new Context object.
func NewContext(b []byte) *Context {
	return &Context{
		prog:        b,
		breakPoints: []int{},
	}
}

// Next returns the next instruction to execute with its parameter if any. After
// its invocation the instruction pointer points to the instruction being
// returned.
func (c *Context) Next() (Instruction, []byte, error) {
	c.ip = c.nextip
	if c.ip >= len(c.prog) {
		return RET, nil, nil
	}
	r := io.NewBinReaderFromBuf(c.prog[c.ip:])

	var instrbyte byte
	r.ReadLE(&instrbyte)
	instr := Instruction(instrbyte)
	c.nextip++

	var numtoread int
	switch instr {
	case PUSHDATA1, SYSCALL:
		var n byte
		r.ReadLE(&n)
		numtoread = int(n)
		c.nextip++
	case PUSHDATA2:
		var n uint16
		r.ReadLE(&n)
		numtoread = int(n)
		c.nextip += 2
	case PUSHDATA4:
		var n uint32
		r.ReadLE(&n)
		if n > MaxItemSize {
			return instr, nil, errors.New("parameter is too big")
		}
		numtoread = int(n)
		c.nextip += 4
	case JMP, JMPIF, JMPIFNOT, CALL:
		numtoread = 2
	case APPCALL, TAILCALL:
		numtoread = 20
	default:
		if instr >= PUSHBYTES1 && instr <= PUSHBYTES75 {
			numtoread = int(instr)
		} else {
			// No parameters, can just return.
			return instr, nil, nil
		}
	}
	parameter := make([]byte, numtoread)
	r.ReadLE(parameter)
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
func (c *Context) CurrInstr() (int, Instruction) {
	return c.ip, Instruction(c.prog[c.ip])
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
