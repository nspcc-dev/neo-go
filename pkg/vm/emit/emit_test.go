package emit

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
)

func TestEmitInt(t *testing.T) {
	buf := new(bytes.Buffer)
	Int(buf, 10)
	assert.Equal(t, opcode.Opcode(buf.Bytes()[0]), opcode.PUSH10)
	buf.Reset()
	Int(buf, 100)
	assert.Equal(t, buf.Bytes()[0], uint8(1))
	assert.Equal(t, buf.Bytes()[1], uint8(100))
	buf.Reset()
	Int(buf, 1000)
	assert.Equal(t, buf.Bytes()[0], uint8(2))
	assert.Equal(t, buf.Bytes()[1:3], []byte{0xe8, 0x03})
}

func TestEmitBool(t *testing.T) {
	buf := new(bytes.Buffer)
	Bool(buf, true)
	Bool(buf, false)
	assert.Equal(t, opcode.Opcode(buf.Bytes()[0]), opcode.PUSH1)
	assert.Equal(t, opcode.Opcode(buf.Bytes()[1]), opcode.PUSH0)
}

func TestEmitString(t *testing.T) {
	buf := new(bytes.Buffer)
	str := "City Of Zion"
	String(buf, str)
	assert.Equal(t, buf.Len(), len(str)+1)
	assert.Equal(t, buf.Bytes()[1:], []byte(str))
}

func TestEmitSyscall(t *testing.T) {
	syscalls := []string{
		"Neo.Runtime.Log",
		"Neo.Runtime.Notify",
		"Neo.Runtime.Whatever",
	}

	buf := new(bytes.Buffer)
	for _, syscall := range syscalls {
		Syscall(buf, syscall)
		assert.Equal(t, opcode.Opcode(buf.Bytes()[0]), opcode.SYSCALL)
		assert.Equal(t, buf.Bytes()[1], uint8(len(syscall)))
		assert.Equal(t, buf.Bytes()[2:], []byte(syscall))
		buf.Reset()
	}
}

func TestEmitCall(t *testing.T) {
	buf := new(bytes.Buffer)
	Call(buf, opcode.JMP, 100)
	assert.Equal(t, opcode.Opcode(buf.Bytes()[0]), opcode.JMP)
	label := binary.LittleEndian.Uint16(buf.Bytes()[1:3])
	assert.Equal(t, label, uint16(100))
}
