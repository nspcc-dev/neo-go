package compiler

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

func TestEmitPush(t *testing.T) {
	sb := &ScriptBuilder{buf: new(bytes.Buffer)}

	if err := sb.emitPush(vm.OpPush1); err != nil {
		t.Fatal(err)
	}
	if sb.buf.Len() != 1 {
		t.Fatalf("expect buffer len of 1 got %d", sb.buf.Len())
	}
}
func TestEmitPushIntNeg(t *testing.T) {
	sb := &ScriptBuilder{buf: new(bytes.Buffer)}
	val := -1
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	if want, have := vm.OpPushM1, vm.OpCode(sb.buf.Bytes()[0]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
}

func TestEmitPushInt0(t *testing.T) {
	sb := &ScriptBuilder{buf: new(bytes.Buffer)}
	val := 0
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	if want, have := vm.OpPushF, vm.OpCode(sb.buf.Bytes()[0]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
}

func TestEmitPushInt1(t *testing.T) {
	sb := &ScriptBuilder{buf: new(bytes.Buffer)}
	val := 1
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	if want, have := vm.OpPush1, vm.OpCode(sb.buf.Bytes()[0]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
}

func TestEmitPushInt100(t *testing.T) {
	x := 100
	bigx := big.NewInt(int64(x))
	t.Log(bigx.Bytes())

	sb := &ScriptBuilder{buf: new(bytes.Buffer)}
	val := 100
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	// len = 1
	if want, have := byte(0x01), byte(sb.buf.Bytes()[0]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
	if want, have := byte(0x64), byte(sb.buf.Bytes()[1]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
}

func TestEmitPush1000(t *testing.T) {
	sb := &ScriptBuilder{buf: new(bytes.Buffer)}
	val := 1000
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	bInt := big.NewInt(int64(val))
	if want, have := byte(len(bInt.Bytes())), byte(sb.buf.Bytes()[0]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
	want := util.ToArrayReverse(bInt.Bytes()) // reverse
	have := sb.buf.Bytes()[1:]
	if bytes.Compare(want, have) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}

func TestEmitPushString(t *testing.T) {
	sb := &ScriptBuilder{buf: new(bytes.Buffer)}
	str := "anthdm"
	if err := sb.emitPushString(str); err != nil {
		t.Fatal(err)
	}
	if want, have := byte(len(str)), sb.buf.Bytes()[0]; want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
	want, have := []byte(str), sb.buf.Bytes()[1:]
	if bytes.Compare(want, have) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}
