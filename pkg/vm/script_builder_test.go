package vm

import (
	"bytes"
	"testing"
)

func TestEmitPush(t *testing.T) {
	sb := &ScriptBuilder{new(bytes.Buffer)}

	if err := sb.emitPush(OpPush1); err != nil {
		t.Fatal(err)
	}
	if sb.buf.Len() != 1 {
		t.Fatalf("expect buffer len of 1 got %d", sb.buf.Len())
	}
}

func TestEmitPushInt(t *testing.T) {
	sb := &ScriptBuilder{new(bytes.Buffer)}

	val := -1
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	if want, have := OpPushM1, OpCode(sb.buf.Bytes()[0]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
	val = 0
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	if want, have := OpPushF, OpCode(sb.buf.Bytes()[1]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
	val = 1
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	if want, have := OpPush1, OpCode(sb.buf.Bytes()[2]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
	val = 17858588558585
	if err := sb.emitPushInt(int64(val)); err != nil {
		t.Fatal(err)
	}
	// sizeof int64 = 8
	if want, have := byte(8), byte(sb.buf.Bytes()[3]); want != have {
		t.Fatalf("expected %v got %v", want, have)
	}
	want := []byte{249, 196, 211, 6, 62, 16, 0, 0}
	have := sb.buf.Bytes()[4:]
	if bytes.Compare(want, have) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}
