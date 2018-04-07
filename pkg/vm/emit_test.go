package vm

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmitInt(t *testing.T) {
	buf := new(bytes.Buffer)
	EmitInt(buf, 10)
	assert.Equal(t, Opcode(buf.Bytes()[0]), Opush10)
	buf.Reset()
	EmitInt(buf, 100)
	assert.Equal(t, buf.Bytes()[0], uint8(1))
	assert.Equal(t, buf.Bytes()[1], uint8(100))
	buf.Reset()
	EmitInt(buf, 1000)
	assert.Equal(t, buf.Bytes()[0], uint8(2))
	assert.Equal(t, buf.Bytes()[1:3], []byte{0xe8, 0x03})
}

func TestEmitBool(t *testing.T) {
	buf := new(bytes.Buffer)
	EmitBool(buf, true)
	EmitBool(buf, false)
	assert.Equal(t, Opcode(buf.Bytes()[0]), Opush1)
	assert.Equal(t, Opcode(buf.Bytes()[1]), Opush0)
}

func TestEmitString(t *testing.T) {
	buf := new(bytes.Buffer)
	str := "City Of Zion"
	EmitString(buf, str)
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
		EmitSyscall(buf, syscall)
		assert.Equal(t, Opcode(buf.Bytes()[0]), Osyscall)
		assert.Equal(t, buf.Bytes()[1], uint8(len(syscall)))
		assert.Equal(t, buf.Bytes()[2:], []byte(syscall))
		buf.Reset()
	}
}

func TestEmitCall(t *testing.T) {
	buf := new(bytes.Buffer)
	EmitCall(buf, Ojmp, 100)
	assert.Equal(t, Opcode(buf.Bytes()[0]), Ojmp)
	label := binary.LittleEndian.Uint16(buf.Bytes()[1:3])
	assert.Equal(t, label, uint16(100))
}
