package emit

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
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
}

// Int emits a int type to the given buffer.
func Int(w *io.BinWriter, i int64) {
	switch {
	case i == -1:
		Opcode(w, opcode.PUSHM1)
	case i == 0:
		Opcode(w, opcode.PUSHF)
	case i > 0 && i < 16:
		val := opcode.Opcode(int(opcode.PUSH1) - 1 + int(i))
		Opcode(w, val)
	default:
		bInt := big.NewInt(i)
		val := IntToBytes(bInt)
		Bytes(w, val)
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
	case n <= int(opcode.PUSHBYTES75):
		Instruction(w, opcode.Opcode(n), b)
		return
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
	buf := make([]byte, len(api)+1)
	buf[0] = byte(len(api))
	copy(buf[1:], api)
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
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, label)
	Instruction(w, op, buf)
}

// AppCall emits an appcall, if tailCall is true, tailCall opcode will be
// emitted instead.
func AppCall(w *io.BinWriter, scriptHash util.Uint160, tailCall bool) {
	op := opcode.APPCALL
	if tailCall {
		op = opcode.TAILCALL
	}
	Instruction(w, op, scriptHash.BytesBE())
}

// AppCallWithOperationAndData emits an appcall with the given operation and data.
func AppCallWithOperationAndData(w *io.BinWriter, scriptHash util.Uint160, operation string, data []byte) {
	Bytes(w, data)
	String(w, operation)
	AppCall(w, scriptHash, false)
}

// AppCallWithOperation emits an appcall with the given operation.
func AppCallWithOperation(w *io.BinWriter, scriptHash util.Uint160, operation string) {
	Bool(w, false)
	String(w, operation)
	AppCall(w, scriptHash, false)
}

func isInstructionJmp(op opcode.Opcode) bool {
	if op == opcode.JMP || op == opcode.JMPIFNOT || op == opcode.JMPIF || op == opcode.CALL {
		return true
	}
	return false
}
