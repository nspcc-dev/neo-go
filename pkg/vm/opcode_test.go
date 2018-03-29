package vm

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushBytes1to75(t *testing.T) {
	buf := new(bytes.Buffer)
	for i := 1; i <= 75; i++ {
		b := randomBytes(i)
		EmitBytes(buf, b)
		vm := load(buf.Bytes())
		vm.Step()

		assert.Equal(t, 1, vm.estack.Len())

		elem := vm.estack.Pop()
		assert.IsType(t, &byteArrayItem{}, elem.value)
		assert.IsType(t, elem.Bytes(), b)
		assert.Equal(t, 0, vm.estack.Len())

		vm.execute(nil, Oret)

		assert.Equal(t, 0, vm.astack.Len())
		assert.Equal(t, 0, vm.istack.Len())
		buf.Reset()
	}
}

func TestPushm1to16(t *testing.T) {
	prog := []byte{}
	for i := int(Opushm1); i <= int(Opush16); i++ {
		if i == 80 {
			continue // opcode layout we got here.
		}
		prog = append(prog, byte(i))
	}

	vm := load(prog)
	for i := int(Opushm1); i <= int(Opush16); i++ {
		if i == 80 {
			continue // nice opcode layout we got here.
		}
		vm.Step()

		elem := vm.estack.Pop()
		assert.IsType(t, &bigIntegerItem{}, elem.value)
		val := i - int(Opush1) + 1
		assert.Equal(t, elem.BigInt().Int64(), int64(val))
	}
}

func TestPushData1(t *testing.T) {

}

func TestPushData2(t *testing.T) {

}

func TestPushData4(t *testing.T) {

}

func load(prog []byte) *VM {
	vm := New(nil)
	vm.istack.PushVal(NewContext(prog))
	return vm
}

func randomBytes(n int) []byte {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return b
}
