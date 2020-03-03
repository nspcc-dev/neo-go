package emit

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
)

func TestEmitInt(t *testing.T) {
	t.Run("minis one", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, -1)
		result := buf.Bytes()
		assert.Len(t, result, 1)
		assert.EqualValues(t, opcode.PUSHM1, result[0])
	})

	t.Run("zero", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, 0)
		result := buf.Bytes()
		assert.Len(t, result, 1)
		assert.EqualValues(t, opcode.PUSH0, result[0])
	})

	t.Run("1-byte int", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, 10)
		result := buf.Bytes()
		assert.EqualValues(t, opcode.PUSH10, result[0])
	})

	t.Run("2-byte int", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, 100)
		result := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHBYTES1, result[0])
		assert.EqualValues(t, 100, result[1])
	})

	t.Run("4-byte int", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, 1000)
		result := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHBYTES2, result[0])
		assert.EqualValues(t, 1000, binary.LittleEndian.Uint16(result[1:3]))
	})
}

func getSlice(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i)
	}

	return data
}

func TestBytes(t *testing.T) {
	t.Run("small slice", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Bytes(buf.BinWriter, []byte{0, 1, 2, 3})

		result := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHBYTES4, result[0])
		assert.EqualValues(t, []byte{0, 1, 2, 3}, result[1:])
	})

	t.Run("slice with len <= 255", func(t *testing.T) {
		const size = 200

		buf := io.NewBufBinWriter()
		Bytes(buf.BinWriter, getSlice(size))

		result := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHDATA1, result[0])
		assert.EqualValues(t, size, result[1])
		assert.Equal(t, getSlice(size), result[2:])
	})

	t.Run("slice with len <= 65535", func(t *testing.T) {
		const size = 60000

		buf := io.NewBufBinWriter()
		Bytes(buf.BinWriter, getSlice(size))

		result := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHDATA2, result[0])
		assert.EqualValues(t, size, binary.LittleEndian.Uint16(result[1:3]))
		assert.Equal(t, getSlice(size), result[3:])
	})

	t.Run("slice with len > 65535", func(t *testing.T) {
		const size = 100000

		buf := io.NewBufBinWriter()
		Bytes(buf.BinWriter, getSlice(size))

		result := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHDATA4, result[0])
		assert.EqualValues(t, size, binary.LittleEndian.Uint32(result[1:5]))
		assert.Equal(t, getSlice(size), result[5:])
	})
}

func TestEmitBool(t *testing.T) {
	buf := io.NewBufBinWriter()
	Bool(buf.BinWriter, true)
	Bool(buf.BinWriter, false)
	result := buf.Bytes()
	assert.Equal(t, opcode.Opcode(result[0]), opcode.PUSH1)
	assert.Equal(t, opcode.Opcode(result[1]), opcode.PUSH0)
}

func TestEmitString(t *testing.T) {
	buf := io.NewBufBinWriter()
	str := "City Of Zion"
	String(buf.BinWriter, str)
	assert.Equal(t, buf.Len(), len(str)+1)
	assert.Equal(t, buf.Bytes()[1:], []byte(str))
}

func TestEmitSyscall(t *testing.T) {
	syscalls := []string{
		"Neo.Runtime.Log",
		"Neo.Runtime.Notify",
		"Neo.Runtime.Whatever",
	}

	buf := io.NewBufBinWriter()
	for _, syscall := range syscalls {
		Syscall(buf.BinWriter, syscall)
		result := buf.Bytes()
		assert.Equal(t, opcode.Opcode(result[0]), opcode.SYSCALL)
		assert.Equal(t, result[1], uint8(len(syscall)))
		assert.Equal(t, result[2:], []byte(syscall))
		buf.Reset()
	}

	t.Run("empty syscall", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Syscall(buf.BinWriter, "")
		assert.Error(t, buf.Err)
	})

	t.Run("empty syscall after error", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		err := errors.New("first error")

		buf.Err = err
		Syscall(buf.BinWriter, "")
		assert.Equal(t, err, buf.Err)
	})
}

func TestJmp(t *testing.T) {
	const label = 0x23

	t.Run("correct", func(t *testing.T) {
		ops := []opcode.Opcode{opcode.JMP, opcode.JMPIF, opcode.JMPIFNOT, opcode.CALL}
		for i := range ops {
			t.Run(ops[i].String(), func(t *testing.T) {
				buf := io.NewBufBinWriter()
				Jmp(buf.BinWriter, ops[i], label)
				assert.NoError(t, buf.Err)

				result := buf.Bytes()
				assert.EqualValues(t, ops[i], result[0])
				assert.EqualValues(t, 0x23, binary.LittleEndian.Uint16(result[1:]))
			})
		}
	})

	t.Run("not a jump instruction", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Jmp(buf.BinWriter, opcode.ABS, label)
		assert.Error(t, buf.Err)
	})

	t.Run("not a jump after error", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		err := errors.New("first error")

		buf.Err = err
		Jmp(buf.BinWriter, opcode.ABS, label)
		assert.Error(t, buf.Err)
	})
}

func TestEmitCall(t *testing.T) {
	buf := io.NewBufBinWriter()
	Call(buf.BinWriter, opcode.JMP, 100)
	result := buf.Bytes()
	assert.Equal(t, opcode.Opcode(result[0]), opcode.JMP)
	label := binary.LittleEndian.Uint16(result[1:3])
	assert.Equal(t, label, uint16(100))
}
