package vm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Emit a VM Opcode with data to the given buffer.
func Emit(w *bytes.Buffer, op Opcode, b []byte) error {
	if err := w.WriteByte(byte(op)); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

// EmitOpcode emits a single VM Opcode the given buffer.
func EmitOpcode(w *bytes.Buffer, op Opcode) error {
	return w.WriteByte(byte(op))
}

// EmitBool emits a bool type the given buffer.
func EmitBool(w *bytes.Buffer, ok bool) error {
	if ok {
		return EmitOpcode(w, Opusht)
	}
	return EmitOpcode(w, Opushf)
}

// EmitInt emits a int type to the given buffer.
func EmitInt(w *bytes.Buffer, i int64) error {
	if i == -1 {
		return EmitOpcode(w, Opushm1)
	}
	if i == 0 {
		return EmitOpcode(w, Opushf)
	}
	if i > 0 && i < 16 {
		val := Opcode((int(Opush1) - 1 + int(i)))
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

	if n <= int(Opushbytes75) {
		return Emit(w, Opcode(n), b)
	} else if n < 0x100 {
		err = Emit(w, Opushdata1, []byte{byte(n)})
	} else if n < 0x10000 {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		err = Emit(w, Opushdata2, buf)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		err = Emit(w, Opushdata4, buf)
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
	buf := make([]byte, len(api)+1)
	buf[0] = byte(len(api))
	copy(buf[1:len(buf)], []byte(api))
	return Emit(w, Osyscall, buf)
}

// EmitCall emits a call Opcode with label to the given buffer.
func EmitCall(w *bytes.Buffer, op Opcode, label int16) error {
	return EmitJmp(w, op, label)
}

// EmitJmp emits a jump Opcode along with label to the given buffer.
func EmitJmp(w *bytes.Buffer, op Opcode, label int16) error {
	if !isOpcodeJmp(op) {
		return fmt.Errorf("opcode %s is not a jump or call type", op)
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	return Emit(w, op, buf)
}

// EmitAppCall emits an appcall, if tailCall is true, tailCall opcode will be
// emitted instead.
func EmitAppCall(w *bytes.Buffer, scriptHash util.Uint160, tailCall bool) error {
	op := Oappcall
	if tailCall {
		op = Otailcall
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

func isOpcodeJmp(op Opcode) bool {
	if op == Ojmp || op == Ojmpifnot || op == Ojmpif || op == Ocall {
		return true
	}
	return false
}
