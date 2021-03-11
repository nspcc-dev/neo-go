package emit

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("big 1-byte int", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, 42)
		result := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHINT8, result[0])
		assert.EqualValues(t, 42, result[1])
	})

	t.Run("2-byte int", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, 300)
		result := buf.Bytes()
		assert.Equal(t, 3, len(result))
		assert.EqualValues(t, opcode.PUSHINT16, result[0])
		assert.EqualValues(t, 300, bigint.FromBytes(result[1:]).Int64())
	})

	t.Run("3-byte int", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, 1<<20)
		result := buf.Bytes()
		assert.Equal(t, 5, len(result))
		assert.EqualValues(t, opcode.PUSHINT32, result[0])
		assert.EqualValues(t, 1<<20, bigint.FromBytes(result[1:]).Int64())
	})

	t.Run("4-byte int", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, 1<<28)
		result := buf.Bytes()
		assert.Equal(t, 5, len(result))
		assert.EqualValues(t, opcode.PUSHINT32, result[0])
		assert.EqualValues(t, 1<<28, bigint.FromBytes(result[1:]).Int64())
	})

	t.Run("negative 3-byte int with padding", func(t *testing.T) {
		const num = -(1 << 23)
		buf := io.NewBufBinWriter()
		Int(buf.BinWriter, num)
		result := buf.Bytes()
		assert.Equal(t, 5, len(result))
		assert.EqualValues(t, opcode.PUSHINT32, result[0])
		assert.EqualValues(t, num, bigint.FromBytes(result[1:]).Int64())
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
		assert.EqualValues(t, opcode.PUSHDATA1, result[0])
		assert.EqualValues(t, 4, result[1])
		assert.EqualValues(t, []byte{0, 1, 2, 3}, result[2:])
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

func TestEmitArray(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		veryBig := new(big.Int).SetUint64(math.MaxUint64)
		veryBig.Add(veryBig, big.NewInt(1))
		Array(buf.BinWriter, big.NewInt(0), veryBig,
			[]interface{}{int64(1), int64(2)}, nil, int64(1), "str", true, []byte{0xCA, 0xFE})
		require.NoError(t, buf.Err)

		res := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHDATA1, res[0])
		assert.EqualValues(t, 2, res[1])
		assert.EqualValues(t, []byte{0xCA, 0xFE}, res[2:4])
		assert.EqualValues(t, opcode.PUSHT, res[4])
		assert.EqualValues(t, opcode.CONVERT, res[5])
		assert.EqualValues(t, stackitem.BooleanT, res[6])
		assert.EqualValues(t, opcode.PUSHDATA1, res[7])
		assert.EqualValues(t, 3, res[8])
		assert.EqualValues(t, []byte("str"), res[9:12])
		assert.EqualValues(t, opcode.PUSH1, res[12])
		assert.EqualValues(t, opcode.PUSHNULL, res[13])
		assert.EqualValues(t, opcode.PUSH2, res[14])
		assert.EqualValues(t, opcode.PUSH1, res[15])
		assert.EqualValues(t, opcode.PUSH2, res[16])
		assert.EqualValues(t, opcode.PACK, res[17])
		assert.EqualValues(t, opcode.PUSHINT128, res[18])
		assert.EqualValues(t, veryBig, bigint.FromBytes(res[19:35]))
		assert.EqualValues(t, opcode.PUSH0, res[35])
	})

	t.Run("empty", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Array(buf.BinWriter)
		require.NoError(t, buf.Err)
		assert.EqualValues(t, []byte{byte(opcode.PUSH0), byte(opcode.PACK)}, buf.Bytes())
	})

	t.Run("invalid type", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Array(buf.BinWriter, struct{}{})
		require.Error(t, buf.Err)
	})
}

func TestEmitBool(t *testing.T) {
	buf := io.NewBufBinWriter()
	Bool(buf.BinWriter, true)
	Bool(buf.BinWriter, false)
	result := buf.Bytes()
	assert.EqualValues(t, opcode.PUSH1, result[0])
	assert.EqualValues(t, opcode.CONVERT, result[1])
	assert.EqualValues(t, stackitem.BooleanT, result[2])
	assert.EqualValues(t, opcode.PUSH0, result[3])
	assert.EqualValues(t, opcode.CONVERT, result[4])
	assert.EqualValues(t, stackitem.BooleanT, result[5])
}

func TestEmitOpcode(t *testing.T) {
	w := io.NewBufBinWriter()
	Opcodes(w.BinWriter, opcode.PUSH1, opcode.NEWMAP)
	result := w.Bytes()
	assert.Equal(t, result, []byte{byte(opcode.PUSH1), byte(opcode.NEWMAP)})
}

func TestEmitString(t *testing.T) {
	buf := io.NewBufBinWriter()
	str := "City Of Zion"
	String(buf.BinWriter, str)
	assert.Equal(t, buf.Len(), len(str)+2)
	assert.Equal(t, buf.Bytes()[2:], []byte(str))
}

func TestEmitSyscall(t *testing.T) {
	syscalls := []string{
		interopnames.SystemRuntimeLog,
		interopnames.SystemRuntimeNotify,
		"System.Runtime.Whatever",
	}

	buf := io.NewBufBinWriter()
	for _, syscall := range syscalls {
		Syscall(buf.BinWriter, syscall)
		result := buf.Bytes()
		assert.Equal(t, 5, len(result))
		assert.Equal(t, opcode.Opcode(result[0]), opcode.SYSCALL)
		assert.Equal(t, binary.LittleEndian.Uint32(result[1:]), interopnames.ToID([]byte(syscall)))
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
