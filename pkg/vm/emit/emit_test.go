package emit

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

func TestEmitBigInt(t *testing.T) {
	t.Run("biggest positive number", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		bi := big.NewInt(1)
		bi.Lsh(bi, 255)
		bi.Sub(bi, big.NewInt(1))

		// sanity check
		require.NotPanics(t, func() { stackitem.NewBigInteger(bi) })

		BigInt(buf.BinWriter, bi)
		require.NoError(t, buf.Err)

		expected := make([]byte, 33)
		expected[0] = byte(opcode.PUSHINT256)
		for i := 1; i < 32; i++ {
			expected[i] = 0xFF
		}
		expected[32] = 0x7F
		require.Equal(t, expected, buf.Bytes())
	})
	t.Run("smallest negative number", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		bi := big.NewInt(-1)
		bi.Lsh(bi, 255)

		// sanity check
		require.NotPanics(t, func() { stackitem.NewBigInteger(bi) })

		BigInt(buf.BinWriter, bi)
		require.NoError(t, buf.Err)

		expected := make([]byte, 33)
		expected[0] = byte(opcode.PUSHINT256)
		expected[32] = 0x80
		require.Equal(t, expected, buf.Bytes())
	})
	t.Run("biggest positive number plus 1", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		bi := big.NewInt(1)
		bi.Lsh(bi, 255)

		// sanity check
		require.Panics(t, func() { stackitem.NewBigInteger(bi) })

		BigInt(buf.BinWriter, bi)
		require.Error(t, buf.Err)

		t.Run("do not clear previous error", func(t *testing.T) {
			buf.Reset()
			expected := errors.New("expected")
			buf.Err = expected
			BigInt(buf.BinWriter, bi)
			require.Equal(t, expected, buf.Err)
		})
	})
	t.Run("smallest negative number minus 1", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		bi := big.NewInt(-1)
		bi.Lsh(bi, 255)
		bi.Sub(bi, big.NewInt(1))

		// sanity check
		require.Panics(t, func() { stackitem.NewBigInteger(bi) })

		BigInt(buf.BinWriter, bi)
		require.Error(t, buf.Err)
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
		var p160 *util.Uint160
		var p256 *util.Uint256
		u160 := util.Uint160{1, 2, 3}
		u256 := util.Uint256{1, 2, 3}
		veryBig := new(big.Int).SetUint64(math.MaxUint64)
		veryBig.Add(veryBig, big.NewInt(1))
		Array(buf.BinWriter,
			uint64(math.MaxUint64),
			uint(math.MaxUint32), // don't use MaxUint to keep test results the same throughout all platforms.
			stackitem.NewMapWithValue([]stackitem.MapElement{
				{
					Key:   stackitem.Make(1),
					Value: stackitem.Make("str1"),
				},
				{
					Key:   stackitem.Make(2),
					Value: stackitem.Make("str2"),
				},
			}),
			stackitem.NewStruct([]stackitem.Item{
				stackitem.Make(4),
				stackitem.Make("str"),
			}),
			&ConvertibleStruct{
				SomeInt:    5,
				SomeString: "str",
			},
			stackitem.Make(5),
			stackitem.Make("str"),
			stackitem.NewArray([]stackitem.Item{
				stackitem.Make(true),
				stackitem.Make("str"),
			}),
			p160, p256, &u160, &u256, u160, u256, big.NewInt(0), veryBig,
			[]any{int64(1), int64(2)}, nil, int64(1), "str", false, true, []byte{0xCA, 0xFE})
		require.NoError(t, buf.Err)

		res := buf.Bytes()
		assert.EqualValues(t, opcode.PUSHDATA1, res[0])
		assert.EqualValues(t, 2, res[1])
		assert.EqualValues(t, []byte{0xCA, 0xFE}, res[2:4])
		assert.EqualValues(t, opcode.PUSHT, res[4])
		assert.EqualValues(t, opcode.PUSHF, res[5])
		assert.EqualValues(t, opcode.PUSHDATA1, res[6])
		assert.EqualValues(t, 3, res[7])
		assert.EqualValues(t, []byte("str"), res[8:11])
		assert.EqualValues(t, opcode.PUSH1, res[11])
		assert.EqualValues(t, opcode.PUSHNULL, res[12])
		assert.EqualValues(t, opcode.PUSH2, res[13])
		assert.EqualValues(t, opcode.PUSH1, res[14])
		assert.EqualValues(t, opcode.PUSH2, res[15])
		assert.EqualValues(t, opcode.PACK, res[16])
		assert.EqualValues(t, opcode.PUSHINT128, res[17])
		assert.EqualValues(t, veryBig, bigint.FromBytes(res[18:34]))
		assert.EqualValues(t, opcode.PUSH0, res[34])
		assert.EqualValues(t, opcode.PUSHDATA1, res[35])
		assert.EqualValues(t, 32, res[36])
		assert.EqualValues(t, u256.BytesBE(), res[37:69])
		assert.EqualValues(t, opcode.PUSHDATA1, res[69])
		assert.EqualValues(t, 20, res[70])
		assert.EqualValues(t, u160.BytesBE(), res[71:91])
		assert.EqualValues(t, opcode.PUSHDATA1, res[91])
		assert.EqualValues(t, 32, res[92])
		assert.EqualValues(t, u256.BytesBE(), res[93:125])
		assert.EqualValues(t, opcode.PUSHDATA1, res[125])
		assert.EqualValues(t, 20, res[126])
		assert.EqualValues(t, u160.BytesBE(), res[127:147])
		assert.EqualValues(t, opcode.PUSHNULL, res[147])
		assert.EqualValues(t, opcode.PUSHNULL, res[148])
		// Array of two stackitems:
		assert.EqualValues(t, opcode.PUSHDATA1, res[149])
		assert.EqualValues(t, 3, res[150])
		assert.EqualValues(t, []byte("str"), res[151:154])
		assert.EqualValues(t, opcode.PUSHT, res[154])
		assert.EqualValues(t, opcode.PUSH2, res[155])
		assert.EqualValues(t, opcode.PACK, res[156])
		// ByteString stackitem ("str"):
		assert.EqualValues(t, opcode.PUSHDATA1, res[157])
		assert.EqualValues(t, 3, res[158])
		assert.EqualValues(t, []byte("str"), res[159:162])
		// Integer stackitem (5):
		assert.EqualValues(t, opcode.PUSH5, res[162])
		// Convertible struct:
		assert.EqualValues(t, opcode.PUSHDATA1, res[163])
		assert.EqualValues(t, 3, res[164])
		assert.EqualValues(t, []byte("str"), res[165:168])
		assert.EqualValues(t, opcode.PUSH5, res[168])
		assert.EqualValues(t, opcode.PUSH2, res[169])
		assert.EqualValues(t, opcode.PACK, res[170])
		// Struct stackitem (4, "str")
		assert.EqualValues(t, opcode.PUSHDATA1, res[171])
		assert.EqualValues(t, 3, res[172])
		assert.EqualValues(t, []byte("str"), res[173:176])
		assert.EqualValues(t, opcode.PUSH4, res[176])
		assert.EqualValues(t, opcode.PUSH2, res[177])
		assert.EqualValues(t, opcode.PACKSTRUCT, res[178])
		// Map stackitem (1:"str1", 2:"str2")
		assert.EqualValues(t, opcode.PUSHDATA1, res[179])
		assert.EqualValues(t, 4, res[180])
		assert.EqualValues(t, []byte("str2"), res[181:185])
		assert.EqualValues(t, opcode.PUSH2, res[185])
		assert.EqualValues(t, opcode.PUSHDATA1, res[186])
		assert.EqualValues(t, 4, res[187])
		assert.EqualValues(t, []byte("str1"), res[188:192])
		assert.EqualValues(t, opcode.PUSH1, res[192])
		assert.EqualValues(t, opcode.PUSH2, res[193])
		assert.EqualValues(t, opcode.PACKMAP, res[194])
		// uint (MaxUint32)
		assert.EqualValues(t, opcode.PUSHINT64, res[195])
		assert.EqualValues(t, []byte{
			0xff, 0xff, 0xff, 0xff,
			0, 0, 0, 0,
		}, res[196:204])
		// uint64 (MaxUint64)
		assert.EqualValues(t, opcode.PUSHINT128, res[204])
		assert.EqualValues(t, []byte{
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
			0, 0, 0, 0,
			0, 0, 0, 0}, res[205:221])

		// Values packing:
		assert.EqualValues(t, opcode.PUSHINT8, res[221])
		assert.EqualValues(t, byte(23), res[222])
		assert.EqualValues(t, opcode.PACK, res[223])

		// Overall script length:
		assert.EqualValues(t, 224, len(res))
	})

	t.Run("empty", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		Array(buf.BinWriter)
		require.NoError(t, buf.Err)
		assert.EqualValues(t, []byte{byte(opcode.NEWARRAY0)}, buf.Bytes())
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
	assert.EqualValues(t, opcode.PUSHT, result[0])
	assert.EqualValues(t, opcode.PUSHF, result[1])
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

func TestEmitStackitem(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		itms := []stackitem.Item{
			stackitem.Make(true),
			stackitem.Make(false),
			stackitem.Make(5),
			stackitem.Make("str"),
			stackitem.Make([]stackitem.Item{
				stackitem.Make(true),
				stackitem.Make([]stackitem.Item{
					stackitem.Make(1),
					stackitem.Make("str"),
				}),
			}),
			stackitem.NewStruct([]stackitem.Item{
				stackitem.Make(true),
				stackitem.Make(7),
			}),
			stackitem.NewMapWithValue([]stackitem.MapElement{
				{
					Key:   stackitem.Make(7),
					Value: stackitem.Make("str1"),
				},
				{
					Key:   stackitem.Make(8),
					Value: stackitem.Make("str2"),
				},
			}),
			stackitem.Null{},
		}
		for _, si := range itms {
			StackItem(buf.BinWriter, si)
		}
		require.NoError(t, buf.Err)
		res := buf.Bytes()

		// Single values:
		assert.EqualValues(t, opcode.PUSHT, res[0])
		assert.EqualValues(t, opcode.PUSHF, res[1])
		assert.EqualValues(t, opcode.PUSH5, res[2])
		assert.EqualValues(t, opcode.PUSHDATA1, res[3])
		assert.EqualValues(t, 3, res[4])
		assert.EqualValues(t, []byte("str"), res[5:8])
		// Nested array:
		assert.EqualValues(t, opcode.PUSHDATA1, res[8])
		assert.EqualValues(t, 3, res[9])
		assert.EqualValues(t, []byte("str"), res[10:13])
		assert.EqualValues(t, opcode.PUSH1, res[13])
		assert.EqualValues(t, opcode.PUSH2, res[14])
		assert.EqualValues(t, opcode.PACK, res[15])
		assert.EqualValues(t, opcode.PUSHT, res[16])
		assert.EqualValues(t, opcode.PUSH2, res[17])
		assert.EqualValues(t, opcode.PACK, res[18])
		// Struct (true, 7):
		assert.EqualValues(t, opcode.PUSH7, res[19])
		assert.EqualValues(t, opcode.PUSHT, res[20])
		assert.EqualValues(t, opcode.PUSH2, res[21])
		assert.EqualValues(t, opcode.PACKSTRUCT, res[22])
		// Map (7:"str1", 8:"str2"):
		assert.EqualValues(t, opcode.PUSHDATA1, res[23])
		assert.EqualValues(t, 4, res[24])
		assert.EqualValues(t, []byte("str2"), res[25:29])
		assert.EqualValues(t, opcode.PUSH8, res[29])
		assert.EqualValues(t, opcode.PUSHDATA1, res[30])
		assert.EqualValues(t, 4, res[31])
		assert.EqualValues(t, []byte("str1"), res[32:36])
		assert.EqualValues(t, opcode.PUSH7, res[36])
		assert.EqualValues(t, opcode.PUSH2, res[37])
		assert.EqualValues(t, opcode.PACKMAP, res[38])
		// Null:
		assert.EqualValues(t, opcode.PUSHNULL, res[39])

		// Overall script length:
		require.Equal(t, 40, len(res))
	})

	t.Run("unsupported", func(t *testing.T) {
		itms := []stackitem.Item{
			stackitem.NewInterop(nil),
			stackitem.NewPointer(123, []byte{123}),
		}
		for _, si := range itms {
			buf := io.NewBufBinWriter()
			StackItem(buf.BinWriter, si)
			require.ErrorIs(t, buf.Err, errors.ErrUnsupported)
		}
	})

	t.Run("invalid any", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		StackItem(buf.BinWriter, StrangeStackItem{})
		actualErr := buf.Err
		require.ErrorIs(t, actualErr, errors.ErrUnsupported)
		require.True(t, strings.Contains(actualErr.Error(), "Any can only be nil"), actualErr.Error())
	})
}

type StrangeStackItem struct{}

var _ = stackitem.Item(StrangeStackItem{})

func (StrangeStackItem) Value() any {
	return struct{}{}
}
func (StrangeStackItem) Type() stackitem.Type {
	return stackitem.AnyT
}
func (StrangeStackItem) String() string {
	panic("TODO")
}
func (StrangeStackItem) Dup() stackitem.Item {
	panic("TODO")
}
func (StrangeStackItem) TryBool() (bool, error) {
	panic("TODO")
}
func (StrangeStackItem) TryBytes() ([]byte, error) {
	panic("TODO")
}
func (StrangeStackItem) TryInteger() (*big.Int, error) {
	panic("TODO")
}
func (StrangeStackItem) Equals(stackitem.Item) bool {
	panic("TODO")
}
func (StrangeStackItem) Convert(stackitem.Type) (stackitem.Item, error) {
	panic("TODO")
}

type ConvertibleStruct struct {
	SomeInt    int
	SomeString string
	err        error
}

var _ = stackitem.Convertible(&ConvertibleStruct{})

func (s *ConvertibleStruct) ToStackItem() (stackitem.Item, error) {
	if s.err != nil {
		return nil, s.err
	}
	return stackitem.NewArray([]stackitem.Item{
		stackitem.Make(s.SomeInt),
		stackitem.Make(s.SomeString),
	}), nil
}

func (s *ConvertibleStruct) FromStackItem(si stackitem.Item) error {
	panic("TODO")
}

func TestEmitConvertible(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		str := &ConvertibleStruct{
			SomeInt:    5,
			SomeString: "str",
		}
		Convertible(buf.BinWriter, str)
		require.NoError(t, buf.Err)
		res := buf.Bytes()

		// The struct itself:
		assert.EqualValues(t, opcode.PUSHDATA1, res[0])
		assert.EqualValues(t, 3, res[1])
		assert.EqualValues(t, []byte("str"), res[2:5])
		assert.EqualValues(t, opcode.PUSH5, res[5])
		assert.EqualValues(t, opcode.PUSH2, res[6])
		assert.EqualValues(t, opcode.PACK, res[7])

		// Overall length:
		assert.EqualValues(t, 8, len(res))
	})

	t.Run("error on conversion", func(t *testing.T) {
		buf := io.NewBufBinWriter()
		expectedErr := errors.New("error on conversion")
		str := &ConvertibleStruct{
			err: expectedErr,
		}
		Convertible(buf.BinWriter, str)
		actualErr := buf.Err
		require.Error(t, actualErr)
		require.ErrorIs(t, actualErr, expectedErr)
	})
}
