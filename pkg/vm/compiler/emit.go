package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

func emit(w *bytes.Buffer, op vm.Opcode, b []byte) error {
	if err := w.WriteByte(byte(op)); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func emitOpcode(w *bytes.Buffer, op vm.Opcode) error {
	return w.WriteByte(byte(op))
}

func emitBool(w *bytes.Buffer, ok bool) error {
	if ok {
		return emitOpcode(w, vm.Opusht)
	}
	return emitOpcode(w, vm.Opushf)
}

func emitInt(w *bytes.Buffer, i int64) error {
	if i == -1 {
		return emitOpcode(w, vm.Opushm1)
	}
	if i == 0 {
		return emitOpcode(w, vm.Opushf)
	}
	if i > 0 && i < 16 {
		val := vm.Opcode((int(vm.Opush1) - 1 + int(i)))
		return emitOpcode(w, val)
	}

	bInt := big.NewInt(i)
	val := util.ToArrayReverse(bInt.Bytes())
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

	if n == 0 {
		// The VM expects a pushf (0x00).
		// Empty strings on the stack for example.
		return emitOpcode(w, vm.Opushf)
	}
	if n <= int(vm.Opushbytes75) {
		return emit(w, vm.Opcode(n), b)
	} else if n < 0x100 {
		err = emit(w, vm.Opushdata1, []byte{byte(n)})
	} else if n < 0x10000 {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		err = emit(w, vm.Opushdata2, buf)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		err = emit(w, vm.Opushdata4, buf)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func emitCall(w *bytes.Buffer, op vm.Opcode, label int16) error {
	return emitJmp(w, op, label)
}

func emitJmp(w *bytes.Buffer, op vm.Opcode, label int16) error {
	if !isOpcodeJmp(op) {
		return fmt.Errorf("opcode %s is not a jump or call type", op)
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	return emit(w, op, buf)
}

func isOpcodeJmp(op vm.Opcode) bool {
	if op == vm.Ojmp || op == vm.Ojmpifnot || op == vm.Ojmpif || op == vm.Ocall {
		return true
	}
	return false
}
