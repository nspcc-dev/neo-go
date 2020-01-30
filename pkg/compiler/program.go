package compiler

import (
	"container/list"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
)

type instruction struct {
	op     opcode.Opcode
	arg    []byte
	labels []int
}

type program struct {
	Err     error
	opcodes *list.List
	labels  map[uint16]uint16
	offsets map[uint16][]int
}

func newProgram() *program {
	return &program{
		opcodes: list.New(),
		labels:  make(map[uint16]uint16),
		offsets: make(map[uint16][]int),
	}
}

func (p *program) emit(op opcode.Opcode, arg []byte) {
	p.opcodes.PushBack(&instruction{
		op:  op,
		arg: arg,
	})
}

func (p *program) emitOpcode(op opcode.Opcode) {
	p.emit(op, nil)
}

func (p *program) emitBool(ok bool) {
	if ok {
		p.emitOpcode(opcode.PUSHT)
		return
	}
	p.emitOpcode(opcode.PUSHF)
}

func (p *program) emitInt(i int64) {
	switch {
	case i == -1:
		p.emitOpcode(opcode.PUSHM1)
	case i == 0:
		p.emitOpcode(opcode.PUSHF)
	case i > 0 && i < 16:
		val := opcode.Opcode(int(opcode.PUSH1) - 1 + int(i))
		p.emitOpcode(val)
	default:
		bInt := big.NewInt(i)
		val := vm.IntToBytes(bInt)
		p.emitBytes(val)
	}
}

func (p *program) emitString(s string) {
	p.emitBytes([]byte(s))
}

func (p *program) emitBytes(b []byte) {
	n := len(b)

	switch {
	case n <= int(opcode.PUSHBYTES75):
		p.emit(opcode.Opcode(n), b)
		return
	case n < 0x100:
		p.emit(opcode.PUSHDATA1, append([]byte{byte(n)}, b...))
	case n < 0x10000:
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		p.emit(opcode.PUSHDATA2, append(buf, b...))
	default:
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		p.emit(opcode.PUSHDATA4, append(buf, b...))
	}
}

// emitSyscall emits the syscall API to the given buffer.
// Syscall API string cannot be 0.
func (p *program) emitSyscall(api string) {
	if len(api) == 0 {
		p.Err = errors.New("syscall api cannot be of length 0")
		return
	}

	buf := make([]byte, len(api)+1)
	buf[0] = byte(len(api))
	copy(buf[1:], api)
	p.emit(opcode.SYSCALL, buf)
}

func (p *program) emitCall(instr opcode.Opcode, label int16) {
	p.emitJmp(instr, label)
}

func (p *program) emitJmp(instr opcode.Opcode, label int16) {
	if !isInstrJmp(instr) {
		panic(fmt.Sprintf("opcode %s is not a jump or call type", instr))
	}

	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(label))
	p.emit(instr, buf)
}

func (p *program) iterate(f func(*list.Element) *list.Element) {
	for elem := p.opcodes.Front(); elem != nil; {
		if elem = f(elem); elem != nil {
			elem = elem.Next()
		}
	}
}

func (p *program) Bytes() []byte {
	w := io.NewBufBinWriter()
	p.iterate(func(elem *list.Element) *list.Element {
		inst := elem.Value.(*instruction)
		// for labels just save current index
		if len(inst.labels) != 0 {
			for _, l := range inst.labels {
				p.labels[uint16(l)] = uint16(w.Len())
			}
			return elem
		}

		emit(w.BinWriter, inst.op, inst.arg)

		// for jump instructions save offset of a label to write
		if isInstrJmp(inst.op) {
			label := binary.LittleEndian.Uint16(inst.arg)
			p.offsets[label] = append(p.offsets[label], w.Len()-2)
		}
		return elem
	})

	buf := w.Bytes()

	// rewrite jump targets
	for label, indices := range p.offsets {
		for _, i := range indices {
			binary.LittleEndian.PutUint16(buf[i:], p.labels[label]-uint16(i)+1)
		}
	}

	return buf
}
