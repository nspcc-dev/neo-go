package vm

import "encoding/binary"

// Context represent the current execution context of the VM.
type Context struct {
	// Instruction pointer.
	ip int

	// The raw program script.
	prog []byte
}

// NewContext return a new Context object.
func NewContext(b []byte) *Context {
	return &Context{
		ip:   -1,
		prog: b,
	}
}

// Next return the next instruction to execute.
func (c *Context) Next() Opcode {
	c.ip++
	return Opcode(c.prog[c.ip])
}

// IP returns the absosulute instruction without taking 0 into account.
// If that program starts the ip = 0 but IP() will return 1, cause its
// the first instruction.
func (c *Context) IP() int {
	return c.ip + 1
}

// Copy returns an new exact copy of c.
func (c *Context) Copy() *Context {
	return &Context{
		ip:   c.ip,
		prog: c.prog,
	}
}

// Value implements StackItem interface.
func (c *Context) Value() interface{} {
	return c
}

func (c *Context) String() string {
	return "execution context"
}

func (c *Context) readUint32() uint32 {
	start, end := c.IP(), c.IP()+4
	val := binary.LittleEndian.Uint32(c.prog[start:end])
	c.ip += 4
	return val
}

func (c *Context) readUint16() uint16 {
	start, end := c.IP(), c.IP()+2
	val := binary.LittleEndian.Uint16(c.prog[start:end])
	c.ip += 2
	return val
}

func (c *Context) readByte() byte {
	start, end := c.IP(), c.IP()+1
	c.ip++
	return c.prog[start:end][0]
}

func (c *Context) readBytes(n int) []byte {
	start, end := c.IP(), c.IP()+n
	out := make([]byte, n)
	copy(out, c.prog[start:end])
	c.ip += n
	return out
}

func (c *Context) readVarBytes() []byte {
	n := c.readByte()
	return c.readBytes(int(n))
}
