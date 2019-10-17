package vm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Emit a VM Instruction with data to the given buffer.
func Emit(w *bytes.Buffer, op Instruction, b []byte) error {
	if err := w.WriteByte(byte(op)); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

// EmitOpcode emits a single VM Instruction the given buffer.
func EmitOpcode(w io.ByteWriter, op Instruction) error {
	return w.WriteByte(byte(op))
}

// EmitBool emits a bool type the given buffer.
func EmitBool(w io.ByteWriter, ok bool) error {
	if ok {
		return EmitOpcode(w, PUSHT)
	}
	return EmitOpcode(w, PUSHF)
}

// EmitInt emits a int type to the given buffer.
func EmitInt(w *bytes.Buffer, i int64) error {
	if i == -1 {
		return EmitOpcode(w, PUSHM1)
	}
	if i == 0 {
		return EmitOpcode(w, PUSHF)
	}
	if i > 0 && i < 16 {
		val := Instruction(int(PUSH1) - 1 + int(i))
		return EmitOpcode(w, val)
	}

	bInt := big.NewInt(i)
	val := util.ArrayReverse(bInt.Bytes())
	return EmitBytes(w, val)
}

// EmitString emits a string to the given buffer.
func EmitString(w *bytes.Buffer, s string) error {
	return EmitBytes(w, []byte(s))
}

// EmitBytes emits a byte array to the given buffer.
func EmitBytes(w *bytes.Buffer, b []byte) error {
	var (
		err error
		n   = len(b)
	)

	if n <= int(PUSHBYTES75) {
		return Emit(w, Instruction(n), b)
	} else if n < 0x100 {
		err = Emit(w, PUSHDATA1, []byte{byte(n)})
	} else if n < 0x10000 {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		err = Emit(w, PUSHDATA2, buf)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		err = Emit(w, PUSHDATA4, buf)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// EmitSyscall emits the syscall API to the given buffer.
// Syscall API string cannot be 0.
func EmitSyscall(w *bytes.Buffer, api string) error {
	if len(api) == 0 {
		return errors.New("syscall api cannot be of length 0")
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, InteropFuncID(api))
	return Emit(w, SYSCALL, buf)
}

// EmitCall emits a call Instruction with label to the given buffer.
func EmitCall(w *bytes.Buffer, op Instruction, label int16) error {
	return EmitJmp(w, op, label)
}

// EmitJmp emits a jump Instruction along with label to the given buffer.
func EmitJmp(w *bytes.Buffer, op Instruction, label int16) error {
	if !isInstructionJmp(op) {
		return fmt.Errorf("opcode %s is not a jump or call type", op.String())
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	return Emit(w, op, buf)
}

// EmitAppCall emits an appcall, if tailCall is true, tailCall opcode will be
// emitted instead.
func EmitAppCall(w *bytes.Buffer, scriptHash util.Uint160, tailCall bool) error {
	op := APPCALL
	if tailCall {
		op = TAILCALL
	}
	return Emit(w, op, scriptHash.Bytes())
}

// EmitAppCallWithOperationAndData emits an appcall with the given operation and data.
func EmitAppCallWithOperationAndData(w *bytes.Buffer, scriptHash util.Uint160, operation string, data []byte) error {
	if err := EmitBytes(w, data); err != nil {
		return err
	}
	if err := EmitString(w, operation); err != nil {
		return err
	}
	return EmitAppCall(w, scriptHash, false)
}

// EmitAppCallWithOperation emits an appcall with the given operation.
func EmitAppCallWithOperation(w *bytes.Buffer, scriptHash util.Uint160, operation string) error {
	if err := EmitBool(w, false); err != nil {
		return err
	}
	if err := EmitString(w, operation); err != nil {
		return err
	}
	return EmitAppCall(w, scriptHash, false)
}

func isInstructionJmp(op Instruction) bool {
	if op == JMP || op == JMPIFNOT || op == JMPIF || op == CALL {
		return true
	}
	return false
}
