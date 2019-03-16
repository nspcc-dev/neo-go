package stack

import (
	"encoding/binary"
	"errors"
)

// Context represent the current execution context of the VM.
// context will be treated as stack item
// and placed onto the invocation stack
type Context struct {
	*abstractItem

	// Instruction pointer.
	ip int

	// The raw program script.
	prog []byte

	// Breakpoints
	breakPoints []int

	// Evaluation Stack
	Estack RandomAccess
}

// NewContext return a new Context object.
func NewContext(b []byte) *Context {
	return &Context{
		abstractItem: &abstractItem{},
		ip:           -1,
		prog:         b,
		breakPoints:  []int{},
	}
}

// Context overrides the default implementation
// to return a context item
func (c *Context) Context() (*Context, error) {
	return c, nil
}

// Next return the next instruction to execute.
func (c *Context) Next() (Instruction, error) {
	c.ip++
	if c.ip >= len(c.prog) {
		return RET, errors.New("program pointer is more than the length of program. RETURNING")
	}
	return Instruction(c.prog[c.ip]), nil
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
	if c.ip < 0 {
		return c.ip, Instruction(0x00)
	}
	return c.ip, Instruction(c.prog[c.ip])
}

// Copy returns an new exact copy of c.
func (c *Context) Copy() *Context {
	return &Context{
		ip:          c.ip,
		prog:        c.prog,
		breakPoints: c.breakPoints,
	}
}

// Program returns the loaded program.
func (c *Context) Program() []byte {
	return c.prog
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

// ReadUint32 reads a uint32 from the script
func (c *Context) ReadUint32() uint32 {
	start, end := c.IP(), c.IP()+4
	if end > len(c.prog) {
		return 0
	}
	val := binary.LittleEndian.Uint32(c.prog[start:end])
	c.ip += 4
	return val
}

// ReadUint16 reads a uint16 from the script
func (c *Context) ReadUint16() uint16 {
	start, end := c.IP(), c.IP()+2
	if end > len(c.prog) {
		return 0
	}
	val := binary.LittleEndian.Uint16(c.prog[start:end])
	c.ip += 2
	return val
}

// ReadByte reads one byte from the script
func (c *Context) ReadByte() (byte, error) {
	byt, err := c.ReadBytes(1)
	if err != nil {
		return 0, err
	}

	return byt[0], nil
}

// ReadBytes will read n bytes from the context
func (c *Context) ReadBytes(n int) ([]byte, error) {
	start, end := c.IP(), c.IP()+n
	if end > len(c.prog) {
		return nil, errors.New("Too many bytes to read, pointer goes past end of program")
	}

	out := make([]byte, n)
	copy(out, c.prog[start:end])
	c.ip += n
	return out, nil
}

func (c *Context) readVarBytes() ([]byte, error) {
	n, err := c.ReadByte()
	if err != nil {
		return nil, err
	}
	return c.ReadBytes(int(n))
}
