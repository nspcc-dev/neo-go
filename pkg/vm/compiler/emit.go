package compiler

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// emit a VM Instruction with data to the given buffer.
func emit(w *io.BinWriter, instr vm.Instruction, b []byte) {
	emitOpcode(w, instr)
	w.WriteBytes(b)
}

// emitOpcode emits a single VM Instruction the given buffer.
func emitOpcode(w *io.BinWriter, instr vm.Instruction) {
	w.WriteLE(byte(instr))
}

// emitBool emits a bool type the given buffer.
func emitBool(w *io.BinWriter, ok bool) {
	if ok {
		emitOpcode(w, vm.PUSHT)
		return
	}
	emitOpcode(w, vm.PUSHF)
}

// emitInt emits a int type to the given buffer.
func emitInt(w *io.BinWriter, i int64) {
	switch {
	case i == -1:
		emitOpcode(w, vm.PUSHM1)
		return
	case i == 0:
		emitOpcode(w, vm.PUSHF)
		return
	case i > 0 && i < 16:
		val := vm.Instruction(int(vm.PUSH1) - 1 + int(i))
		emitOpcode(w, val)
		return
	}

	bInt := big.NewInt(i)
	val := util.ArrayReverse(bInt.Bytes())
	emitBytes(w, val)
}

// emitString emits a string to the given buffer.
func emitString(w *io.BinWriter, s string) {
	emitBytes(w, []byte(s))
}

// emitBytes emits a byte array to the given buffer.
func emitBytes(w *io.BinWriter, b []byte) {
	n := len(b)

	switch {
	case n <= int(vm.PUSHBYTES75):
		emit(w, vm.Instruction(n), b)
		return
	case n < 0x100:
		emit(w, vm.PUSHDATA1, []byte{byte(n)})
	case n < 0x10000:
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		emit(w, vm.PUSHDATA2, buf)
	default:
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		emit(w, vm.PUSHDATA4, buf)
		if w.Err != nil {
			return
		}
	}

	w.WriteBytes(b)
}

// emitSyscall emits the syscall API to the given buffer.
// Syscall API string cannot be 0.
func emitSyscall(w *io.BinWriter, api string) {
	if len(api) == 0 {
		w.Err = errors.New("syscall api cannot be of length 0")
		return
	}
	buf := make([]byte, len(api)+1)
	buf[0] = byte(len(api))
	copy(buf[1:], api)
	emit(w, vm.SYSCALL, buf)
}

// emitCall emits a call Instruction with label to the given buffer.
func emitCall(w *io.BinWriter, instr vm.Instruction, label int16) {
	emitJmp(w, instr, label)
}

// emitJmp emits a jump Instruction along with label to the given buffer.
func emitJmp(w *io.BinWriter, instr vm.Instruction, label int16) {
	if !isInstrJmp(instr) {
		w.Err = fmt.Errorf("opcode %s is not a jump or call type", instr)
		return
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	emit(w, instr, buf)
}

func isInstrJmp(instr vm.Instruction) bool {
	if instr == vm.JMP || instr == vm.JMPIFNOT || instr == vm.JMPIF || instr == vm.CALL {
		return true
	}
	return false
}
