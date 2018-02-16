package newcompiler

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
)

func emit(w *bytes.Buffer, op Opcode, b []byte) error {
	if err := w.WriteByte(byte(op)); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func emitOpcode(w *bytes.Buffer, op Opcode) error {
	return w.WriteByte(byte(op))
}

func emitBool(w *bytes.Buffer, ok bool) error {
	if ok {
		return emitOpcode(w, Opusht)
	}
	return emitOpcode(w, Opushf)
}

func emitInt(w *bytes.Buffer, i int64) error {
	if i == -1 {
		return emitOpcode(w, Opushm1)
	}
	if i == 0 {
		return emitOpcode(w, Opushf)
	}
	if i > 0 && i < 16 {
		val := Opcode((int(Opush1) - 1 + int(i)))
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
		return errors.New("cannot emit 0 bytes")
	}
	if n <= int(Opushbytes75) {
		return emit(w, Opcode(n), b)
	} else if n < 0x100 {
		err = emit(w, Opushdata1, []byte{byte(n)})
	} else if n < 0x10000 {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		err = emit(w, Opushdata2, buf)
	} else {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		err = emit(w, Opushdata4, buf)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func emitJmp(w *bytes.Buffer, op Opcode, label int16) error {
	if !isOpcodeJmp(op) {
		return fmt.Errorf("opcode %s is not a jump type", op)
	}
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	return emit(w, op, buf)
}

func isOpcodeJmp(op Opcode) bool {
	if op == Ojmp || op == Ojmpifnot || op == Ojmpif {
		return true
	}
	return false
}
