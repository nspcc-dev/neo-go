package stack

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Builder follows the builder pattern and will be used to build scripts
type Builder struct {
	w   *bytes.Buffer
	err error
}

// NewBuilder returns a new builder object
func NewBuilder() *Builder {
	return &Builder{
		w:   &bytes.Buffer{},
		err: nil,
	}
}

// Bytes returns the byte representation of the built buffer
func (br *Builder) Bytes() []byte {
	return br.w.Bytes()
}

// Emit a VM Opcode with data to the given buffer.
func (br *Builder) Emit(op Instruction, b []byte) *Builder {
	if br.err != nil {
		return br
	}
	br.err = br.w.WriteByte(byte(op))
	_, br.err = br.w.Write(b)
	return br
}

// EmitOpcode emits a single VM Opcode the given buffer.
func (br *Builder) EmitOpcode(op Instruction) *Builder {
	if br.err != nil {
		return br
	}
	br.err = br.w.WriteByte(byte(op))
	return br
}

// EmitBool emits a bool type the given buffer.
func (br *Builder) EmitBool(ok bool) *Builder {
	if br.err != nil {
		return br
	}
	op := PUSHT
	if !ok {
		op = PUSHF
	}
	return br.EmitOpcode(op)
}

// EmitInt emits a int type to the given buffer.
func (br *Builder) EmitInt(i int64) *Builder {
	if br.err != nil {
		return br
	}
	if i == -1 {
		return br.EmitOpcode(PUSHM1)
	}
	if i == 0 {
		return br.EmitOpcode(PUSHF)
	}
	if i > 0 && i < 16 {
		val := Instruction(int(PUSH1) - 1 + int(i))
		return br.EmitOpcode(val)
	}

	bInt := big.NewInt(i)
	val := reverse(bInt.Bytes())
	return br.EmitBytes(val)
}

// EmitString emits a string to the given buffer.
func (br *Builder) EmitString(s string) *Builder {
	if br.err != nil {
		return br
	}
	return br.EmitBytes([]byte(s))
}

// EmitBytes emits a byte array to the given buffer.
func (br *Builder) EmitBytes(b []byte) *Builder {
	if br.err != nil {
		return br
	}
	var (
		n = len(b)
	)

	if n <= int(PUSHBYTES75) {
		return br.Emit(Instruction(n), b)
	} else if n < 0x100 {
		br.Emit(PUSHDATA1, []byte{byte(n)})
	} else if n < 0x10000 {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		br.Emit(PUSHDATA2, buf)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		br.Emit(PUSHDATA4, buf)
	}
	_, br.err = br.w.Write(b)
	return br
}

// EmitSyscall emits the syscall API to the given buffer.
// Syscall API string cannot be 0.
func (br *Builder) EmitSyscall(api string) *Builder {
	if br.err != nil {
		return br
	}
	if len(api) == 0 {
		br.err = errors.New("syscall api cannot be of length 0")
	}
	buf := make([]byte, len(api)+1)
	buf[0] = byte(len(api))
	copy(buf[1:], []byte(api))
	return br.Emit(SYSCALL, buf)
}

// EmitCall emits a call Opcode with label to the given buffer.
func (br *Builder) EmitCall(op Instruction, label int16) *Builder {
	return br.EmitJmp(op, label)
}

// EmitJmp emits a jump Opcode along with label to the given buffer.
func (br *Builder) EmitJmp(op Instruction, label int16) *Builder {
	if !isOpcodeJmp(op) {
		br.err = fmt.Errorf("opcode %d is not a jump or call type", op)
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	return br.Emit(op, buf)
}

// EmitAppCall emits an appcall, if tailCall is true, tailCall opcode will be
// emitted instead.
func (br *Builder) EmitAppCall(scriptHash util.Uint160, tailCall bool) *Builder {
	op := APPCALL
	if tailCall {
		op = TAILCALL
	}
	return br.Emit(op, scriptHash.Bytes())
}

// EmitAppCallWithOperationAndData emits an appcall with the given operation and data.
func (br *Builder) EmitAppCallWithOperationAndData(w *bytes.Buffer, scriptHash util.Uint160, operation string, data []byte) *Builder {
	br.EmitBytes(data)
	br.EmitString(operation)
	return br.EmitAppCall(scriptHash, false)
}

// EmitAppCallWithOperation emits an appcall with the given operation.
func (br *Builder) EmitAppCallWithOperation(scriptHash util.Uint160, operation string) *Builder {
	br.EmitBool(false)
	br.EmitString(operation)
	return br.EmitAppCall(scriptHash, false)
}

func isOpcodeJmp(op Instruction) bool {
	if op == JMP || op == JMPIFNOT || op == JMPIF || op == CALL {
		return true
	}
	return false
}
