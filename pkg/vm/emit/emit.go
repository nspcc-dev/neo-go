package emit

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
)

// Instruction emits a VM Instruction with data to the given buffer.
func Instruction(w *bytes.Buffer, op opcode.Opcode, b []byte) error {
	if err := w.WriteByte(byte(op)); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

// Opcode emits a single VM Instruction without arguments to the given buffer.
func Opcode(w io.ByteWriter, op opcode.Opcode) error {
	return w.WriteByte(byte(op))
}

// Bool emits a bool type the given buffer.
func Bool(w io.ByteWriter, ok bool) error {
	if ok {
		return Opcode(w, opcode.PUSHT)
	}
	return Opcode(w, opcode.PUSHF)
}

// Int emits a int type to the given buffer.
func Int(w *bytes.Buffer, i int64) error {
	if i == -1 {
		return Opcode(w, opcode.PUSHM1)
	}
	if i == 0 {
		return Opcode(w, opcode.PUSHF)
	}
	if i > 0 && i < 16 {
		val := opcode.Opcode(int(opcode.PUSH1) - 1 + int(i))
		return Opcode(w, val)
	}

	bInt := big.NewInt(i)
	val := IntToBytes(bInt)
	return Bytes(w, val)
}

// String emits a string to the given buffer.
func String(w *bytes.Buffer, s string) error {
	return Bytes(w, []byte(s))
}

// Bytes emits a byte array to the given buffer.
func Bytes(w *bytes.Buffer, b []byte) error {
	var (
		err error
		n   = len(b)
	)

	if n <= int(opcode.PUSHBYTES75) {
		return Instruction(w, opcode.Opcode(n), b)
	} else if n < 0x100 {
		err = Instruction(w, opcode.PUSHDATA1, []byte{byte(n)})
	} else if n < 0x10000 {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		err = Instruction(w, opcode.PUSHDATA2, buf)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		err = Instruction(w, opcode.PUSHDATA4, buf)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// Syscall emits the syscall API to the given buffer.
// Syscall API string cannot be 0.
func Syscall(w *bytes.Buffer, api string) error {
	if len(api) == 0 {
		return errors.New("syscall api cannot be of length 0")
	}
	buf := make([]byte, len(api)+1)
	buf[0] = byte(len(api))
	copy(buf[1:], api)
	return Instruction(w, opcode.SYSCALL, buf)
}

// Call emits a call Instruction with label to the given buffer.
func Call(w *bytes.Buffer, op opcode.Opcode, label int16) error {
	return Jmp(w, op, label)
}

// Jmp emits a jump Instruction along with label to the given buffer.
func Jmp(w *bytes.Buffer, op opcode.Opcode, label int16) error {
	if !isInstructionJmp(op) {
		return fmt.Errorf("opcode %s is not a jump or call type", op.String())
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	return Instruction(w, op, buf)
}

// AppCall emits an appcall, if tailCall is true, tailCall opcode will be
// emitted instead.
func AppCall(w *bytes.Buffer, scriptHash util.Uint160, tailCall bool) error {
	op := opcode.APPCALL
	if tailCall {
		op = opcode.TAILCALL
	}
	return Instruction(w, op, scriptHash.BytesBE())
}

// AppCallWithOperationAndData emits an appcall with the given operation and data.
func AppCallWithOperationAndData(w *bytes.Buffer, scriptHash util.Uint160, operation string, data []byte) error {
	if err := Bytes(w, data); err != nil {
		return err
	}
	if err := String(w, operation); err != nil {
		return err
	}
	return AppCall(w, scriptHash, false)
}

// AppCallWithOperation emits an appcall with the given operation.
func AppCallWithOperation(w *bytes.Buffer, scriptHash util.Uint160, operation string) error {
	if err := Bool(w, false); err != nil {
		return err
	}
	if err := String(w, operation); err != nil {
		return err
	}
	return AppCall(w, scriptHash, false)
}

func isInstructionJmp(op opcode.Opcode) bool {
	if op == opcode.JMP || op == opcode.JMPIFNOT || op == opcode.JMPIF || op == opcode.CALL {
		return true
	}
	return false
}
