package compiler

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

func emit(w *bytes.Buffer, instr vm.Instruction, b []byte) error {
	if err := w.WriteByte(byte(instr)); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func emitOpcode(w io.ByteWriter, instr vm.Instruction) error {
	return w.WriteByte(byte(instr))
}

func emitBool(w io.ByteWriter, ok bool) error {
	if ok {
		return emitOpcode(w, vm.PUSHT)
	}
	return emitOpcode(w, vm.PUSHF)
}

func emitInt(w *bytes.Buffer, i int64) error {
	if i == -1 {
		return emitOpcode(w, vm.PUSHM1)
	}
	if i == 0 {
		return emitOpcode(w, vm.PUSHF)
	}
	if i > 0 && i < 16 {
		val := vm.Instruction(int(vm.PUSH1) - 1 + int(i))
		return emitOpcode(w, val)
	}

	bInt := big.NewInt(i)
	val := util.ArrayReverse(bInt.Bytes())
	return emitBytes(w, val)
}

func emitString(w *bytes.Buffer, s string) error {
	return emitBytes(w, []byte(s))
}

func emitBytes(w *bytes.Buffer, b []byte) error {
	var (
		err error
		n   = len(b)
	)

	switch {
	case n <= int(vm.PUSHBYTES75):
		return emit(w, vm.Instruction(n), b)
	case n < 0x100:
		err = emit(w, vm.PUSHDATA1, []byte{byte(n)})
	case n < 0x10000:
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		err = emit(w, vm.PUSHDATA2, buf)
	default:
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		err = emit(w, vm.PUSHDATA4, buf)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func emitSyscall(w *bytes.Buffer, api string) error {
	if len(api) == 0 {
		return errors.New("syscall api cannot be of length 0")
	}
	buf := make([]byte, len(api)+1)
	buf[0] = byte(len(api))
	copy(buf[1:], api)
	return emit(w, vm.SYSCALL, buf)
}

func emitCall(w *bytes.Buffer, instr vm.Instruction, label int16) error {
	return emitJmp(w, instr, label)
}

func emitJmp(w *bytes.Buffer, instr vm.Instruction, label int16) error {
	if !isInstrJmp(instr) {
		return fmt.Errorf("opcode %s is not a jump or call type", instr)
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	return emit(w, instr, buf)
}

func isInstrJmp(instr vm.Instruction) bool {
	if instr == vm.JMP || instr == vm.JMPIFNOT || instr == vm.JMPIF || instr == vm.CALL {
		return true
	}
	return false
}
