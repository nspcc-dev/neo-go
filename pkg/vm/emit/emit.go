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
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Instruction emits a VM Instruction with data to the given buffer.
func Instruction(w *io.BinWriter, op opcode.Opcode, b []byte) {
	w.WriteB(byte(op))
	w.WriteBytes(b)
}

// Opcode emits a single VM Instruction without arguments to the given buffer.
func Opcode(w *io.BinWriter, op opcode.Opcode) {
	w.WriteB(byte(op))
}

// Bool emits a bool type the given buffer.
func Bool(w *io.BinWriter, ok bool) {
	if ok {
		Opcode(w, opcode.PUSHT)
		return
	}
	Opcode(w, opcode.PUSHF)
	Instruction(w, opcode.CONVERT, []byte{byte(stackitem.BooleanT)})
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

// Int emits a int type to the given buffer.
func Int(w *io.BinWriter, i int64) {
	switch {
	case i == -1:
		Opcode(w, opcode.PUSHM1)
	case i >= 0 && i < 16:
		val := opcode.Opcode(int(opcode.PUSH1) - 1 + int(i))
		Opcode(w, val)
	default:
		buf := bigint.ToPreallocatedBytes(big.NewInt(i), make([]byte, 0, 32))
		// l != 0 becase of switch
		padSize := byte(8 - bits.LeadingZeros8(byte(len(buf)-1)))
		Opcode(w, opcode.PUSHINT8+opcode.Opcode(padSize))
		w.WriteBytes(padRight(1<<padSize, buf))
	}
}

// Array emits array of elements to the given buffer.
func Array(w *io.BinWriter, es ...interface{}) {
	for i := len(es) - 1; i >= 0; i-- {
		switch e := es[i].(type) {
		case int64:
			Int(w, e)
		case string:
			String(w, e)
		case util.Uint160:
			Bytes(w, e.BytesBE())
		case []byte:
			Bytes(w, e)
		case bool:
			Bool(w, e)
		default:
			if es[i] != nil {
				w.Err = errors.New("unsupported type")
				return
			}
			Opcode(w, opcode.PUSHNULL)
		}
	}
	Int(w, int64(len(es)))
	Opcode(w, opcode.PACK)
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

// Call emits a call Instruction with label to the given buffer.
func Call(w *io.BinWriter, op opcode.Opcode, label uint16) {
	Jmp(w, op, label)
}

// Jmp emits a jump Instruction along with label to the given buffer.
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

// AppCall emits call to provided contract.
func AppCall(w *io.BinWriter, scriptHash util.Uint160) {
	Bytes(w, scriptHash.BytesBE())
	Syscall(w, interopnames.SystemContractCall)
}

// AppCallWithOperationAndArgs emits an APPCALL with the given operation and arguments.
func AppCallWithOperationAndArgs(w *io.BinWriter, scriptHash util.Uint160, operation string, args ...interface{}) {
	Array(w, args...)
	String(w, operation)
	AppCall(w, scriptHash)
}

func isInstructionJmp(op opcode.Opcode) bool {
	return opcode.JMP <= op && op <= opcode.CALLL || op == opcode.ENDTRYL
}
