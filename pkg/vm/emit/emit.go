package emit

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"math/bits"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Instruction emits a VM Instruction with data to the given buffer.
func Instruction(w *io.BinWriter, op opcode.Opcode, b []byte) {
	w.WriteB(byte(op))
	w.WriteBytes(b)
}

// Opcodes emits a single VM Instruction without arguments to the given buffer.
func Opcodes(w *io.BinWriter, ops ...opcode.Opcode) {
	for _, op := range ops {
		w.WriteB(byte(op))
	}
}

// Bool emits a bool type to the given buffer.
func Bool(w *io.BinWriter, ok bool) {
	var opVal = opcode.PUSHT
	if !ok {
		opVal = opcode.PUSHF
	}
	Opcodes(w, opVal)
}

func padRight(s int, buf []byte) []byte {
	l := len(buf)
	buf = buf[:s]
	if buf[l-1]&0x80 != 0 {
		for i := l; i < s; i++ {
			buf[i] = 0xFF
		}
	}
	return buf
}

// Int emits an int type to the given buffer.
func Int(w *io.BinWriter, i int64) {
	if smallInt(w, i) {
		return
	}
	bigInt(w, big.NewInt(i), false)
}

// BigInt emits a big-integer to the given buffer.
func BigInt(w *io.BinWriter, n *big.Int) {
	bigInt(w, n, true)
}

func smallInt(w *io.BinWriter, i int64) bool {
	switch {
	case i == -1:
		Opcodes(w, opcode.PUSHM1)
	case i >= 0 && i < 16:
		val := opcode.Opcode(int(opcode.PUSH0) + int(i))
		Opcodes(w, val)
	default:
		return false
	}
	return true
}

func bigInt(w *io.BinWriter, n *big.Int, trySmall bool) {
	if w.Err != nil {
		return
	}
	if trySmall && n.IsInt64() && smallInt(w, n.Int64()) {
		return
	}

	if err := stackitem.CheckIntegerSize(n); err != nil {
		w.Err = err
		return
	}

	buf := bigint.ToPreallocatedBytes(n, make([]byte, 0, 32))
	if len(buf) == 0 {
		Opcodes(w, opcode.PUSH0)
		return
	}
	padSize := byte(8 - bits.LeadingZeros8(byte(len(buf)-1)))
	Opcodes(w, opcode.PUSHINT8+opcode.Opcode(padSize))
	w.WriteBytes(padRight(1<<padSize, buf))
}

// Array emits an array of elements to the given buffer. It accepts everything that
// Any accepts.
func Array(w *io.BinWriter, es ...any) {
	if len(es) == 0 {
		Opcodes(w, opcode.NEWARRAY0)
		return
	}
	for i := len(es) - 1; i >= 0; i-- {
		Any(w, es[i])
	}
	Int(w, int64(len(es)))
	Opcodes(w, opcode.PACK)
}

// Any emits element if supported. It accepts elements of the following types:
//   - int8, int16, int32, int64, int
//   - uint8, uint16, uint32, uint64, uint
//   - *big.Int
//   - string, []byte
//   - util.Uint160, *util.Uint160, util.Uint256, *util.Uint256
//   - bool
//   - stackitem.Convertible, stackitem.Item
//   - nil
//   - []any
func Any(w *io.BinWriter, something any) {
	switch e := something.(type) {
	case []any:
		Array(w, e...)
	case int64:
		Int(w, e)
	case uint64:
		BigInt(w, new(big.Int).SetUint64(e))
	case int32:
		Int(w, int64(e))
	case uint32:
		Int(w, int64(e))
	case int16:
		Int(w, int64(e))
	case uint16:
		Int(w, int64(e))
	case int8:
		Int(w, int64(e))
	case uint8:
		Int(w, int64(e))
	case int:
		Int(w, int64(e))
	case uint:
		BigInt(w, new(big.Int).SetUint64(uint64(e)))
	case *big.Int:
		BigInt(w, e)
	case string:
		String(w, e)
	case util.Uint160:
		Bytes(w, e.BytesBE())
	case util.Uint256:
		Bytes(w, e.BytesBE())
	case *util.Uint160:
		if e == nil {
			Opcodes(w, opcode.PUSHNULL)
		} else {
			Bytes(w, e.BytesBE())
		}
	case *util.Uint256:
		if e == nil {
			Opcodes(w, opcode.PUSHNULL)
		} else {
			Bytes(w, e.BytesBE())
		}
	case []byte:
		Bytes(w, e)
	case bool:
		Bool(w, e)
	case stackitem.Convertible:
		Convertible(w, e)
	case stackitem.Item:
		StackItem(w, e)
	default:
		if something != nil {
			w.Err = fmt.Errorf("unsupported type: %T", e)
			return
		}
		Opcodes(w, opcode.PUSHNULL)
	}
}

// Convertible converts provided stackitem.Convertible to the stackitem.Item and
// emits the item to the given buffer.
func Convertible(w *io.BinWriter, c stackitem.Convertible) {
	si, err := c.ToStackItem()
	if err != nil {
		w.Err = fmt.Errorf("failed to convert stackitem.Convertible to stackitem: %w", err)
		return
	}
	StackItem(w, si)
}

// StackItem emits provided stackitem.Item to the given buffer.
func StackItem(w *io.BinWriter, si stackitem.Item) {
	switch t := si.Type(); t {
	case stackitem.AnyT:
		if si.Value() == nil {
			Opcodes(w, opcode.PUSHNULL)
		} else {
			w.Err = fmt.Errorf("only nil value supported for %s", t)
			return
		}
	case stackitem.BooleanT:
		Bool(w, si.Value().(bool))
	case stackitem.IntegerT:
		BigInt(w, si.Value().(*big.Int))
	case stackitem.ByteArrayT, stackitem.BufferT:
		Bytes(w, si.Value().([]byte))
	case stackitem.ArrayT:
		arr := si.Value().([]stackitem.Item)
		arrAny := make([]any, len(arr))
		for i := range arr {
			arrAny[i] = arr[i]
		}
		Array(w, arrAny...)
	case stackitem.StructT:
		arr := si.Value().([]stackitem.Item)
		for i := len(arr) - 1; i >= 0; i-- {
			StackItem(w, arr[i])
		}

		Int(w, int64(len(arr)))
		Opcodes(w, opcode.PACKSTRUCT)
	case stackitem.MapT:
		arr := si.Value().([]stackitem.MapElement)
		for i := len(arr) - 1; i >= 0; i-- {
			StackItem(w, arr[i].Value)
			StackItem(w, arr[i].Key)
		}

		Int(w, int64(len(arr)))
		Opcodes(w, opcode.PACKMAP)
	default:
		w.Err = fmt.Errorf("%s is unsuppoted", t)
		return
	}
}

// String emits a string to the given buffer.
func String(w *io.BinWriter, s string) {
	Bytes(w, []byte(s))
}

// Bytes emits a byte array to the given buffer.
func Bytes(w *io.BinWriter, b []byte) {
	var n = len(b)

	switch {
	case n < 0x100:
		Instruction(w, opcode.PUSHDATA1, []byte{byte(n)})
	case n < 0x10000:
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		Instruction(w, opcode.PUSHDATA2, buf)
	default:
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		Instruction(w, opcode.PUSHDATA4, buf)
	}
	w.WriteBytes(b)
}

// Syscall emits the syscall API to the given buffer.
// Syscall API string cannot be 0.
func Syscall(w *io.BinWriter, api string) {
	if w.Err != nil {
		return
	} else if len(api) == 0 {
		w.Err = errors.New("syscall api cannot be of length 0")
		return
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, interopnames.ToID([]byte(api)))
	Instruction(w, opcode.SYSCALL, buf)
}

// Call emits a call Instruction with the label to the given buffer.
func Call(w *io.BinWriter, op opcode.Opcode, label uint16) {
	Jmp(w, op, label)
}

// Jmp emits a jump Instruction along with the label to the given buffer.
func Jmp(w *io.BinWriter, op opcode.Opcode, label uint16) {
	if w.Err != nil {
		return
	} else if !isInstructionJmp(op) {
		w.Err = fmt.Errorf("opcode %s is not a jump or call type", op.String())
		return
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint16(buf, label)
	Instruction(w, op, buf)
}

// AppCallNoArgs emits a call to the provided contract.
func AppCallNoArgs(w *io.BinWriter, scriptHash util.Uint160, operation string, f callflag.CallFlag) {
	Int(w, int64(f))
	String(w, operation)
	Bytes(w, scriptHash.BytesBE())
	Syscall(w, interopnames.SystemContractCall)
}

// AppCall emits SYSCALL with System.Contract.Call parameter for given contract, operation, call flag and arguments.
func AppCall(w *io.BinWriter, scriptHash util.Uint160, operation string, f callflag.CallFlag, args ...any) {
	Array(w, args...)
	AppCallNoArgs(w, scriptHash, operation, f)
}

// CheckSig emits a single-key verification script using given []bytes as a key.
// It does not check for key correctness, so you can get an invalid script if the
// data passed is not really a public key.
func CheckSig(w *io.BinWriter, key []byte) {
	Bytes(w, key)
	Syscall(w, interopnames.SystemCryptoCheckSig)
}

func isInstructionJmp(op opcode.Opcode) bool {
	return opcode.JMP <= op && op <= opcode.CALLL || op == opcode.ENDTRYL
}
