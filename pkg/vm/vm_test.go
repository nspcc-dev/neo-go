package vm

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"math/rand"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestInteropHook(t *testing.T) {
	v := New(ModeMute)
	v.RegisterInteropFunc("foo", func(evm *VM) error {
		evm.Estack().PushVal(1)
		return nil
	})

	buf := new(bytes.Buffer)
	EmitSyscall(buf, "foo")
	EmitOpcode(buf, RET)
	v.Load(buf.Bytes())
	v.Run()
	assert.Equal(t, false, v.state.HasFlag(faultState))
	assert.Equal(t, 1, v.estack.Len())
	assert.Equal(t, big.NewInt(1), v.estack.Pop().value.Value())
}

func TestRegisterInterop(t *testing.T) {
	v := New(ModeMute)
	currRegistered := len(v.interop)
	v.RegisterInteropFunc("foo", func(evm *VM) error { return nil })
	assert.Equal(t, currRegistered+1, len(v.interop))
	_, ok := v.interop["foo"]
	assert.Equal(t, true, ok)
}

func TestPushBytes1to75(t *testing.T) {
	buf := new(bytes.Buffer)
	for i := 1; i <= 75; i++ {
		b := randomBytes(i)
		EmitBytes(buf, b)
		vm := load(buf.Bytes())
		vm.Step()

		assert.Equal(t, 1, vm.estack.Len())

		elem := vm.estack.Pop()
		assert.IsType(t, &ByteArrayItem{}, elem.value)
		assert.IsType(t, elem.Bytes(), b)
		assert.Equal(t, 0, vm.estack.Len())

		vm.execute(nil, RET)

		assert.Equal(t, 0, vm.astack.Len())
		assert.Equal(t, 0, vm.istack.Len())
		buf.Reset()
	}
}

func TestPushBytesNoParam(t *testing.T) {
	prog := make([]byte, 1)
	prog[0] = byte(PUSHBYTES1)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushBytesShort(t *testing.T) {
	prog := make([]byte, 10)
	prog[0] = byte(PUSHBYTES10) // but only 9 left in the `prog`
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushm1to16(t *testing.T) {
	var prog []byte
	for i := int(PUSHM1); i <= int(PUSH16); i++ {
		if i == 80 {
			continue // opcode layout we got here.
		}
		prog = append(prog, byte(i))
	}

	vm := load(prog)
	for i := int(PUSHM1); i <= int(PUSH16); i++ {
		if i == 80 {
			continue // nice opcode layout we got here.
		}
		vm.Step()

		elem := vm.estack.Pop()
		assert.IsType(t, &BigIntegerItem{}, elem.value)
		val := i - int(PUSH1) + 1
		assert.Equal(t, elem.BigInt().Int64(), int64(val))
	}
}

func TestPushData1BadNoN(t *testing.T) {
	prog := []byte{byte(PUSHDATA1)}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushData1BadN(t *testing.T) {
	prog := []byte{byte(PUSHDATA1), 1}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushData1Good(t *testing.T) {
	prog := makeProgram(PUSHDATA1, 3, 1, 2, 3)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte{1, 2, 3}, vm.estack.Pop().Bytes())
}

func TestPushData2BadNoN(t *testing.T) {
	prog := []byte{byte(PUSHDATA2)}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushData2ShortN(t *testing.T) {
	prog := []byte{byte(PUSHDATA2), 0}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushData2BadN(t *testing.T) {
	prog := []byte{byte(PUSHDATA2), 1, 0}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushData2Good(t *testing.T) {
	prog := makeProgram(PUSHDATA2, 3, 0, 1, 2, 3)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte{1, 2, 3}, vm.estack.Pop().Bytes())
}

func TestPushData4BadNoN(t *testing.T) {
	prog := []byte{byte(PUSHDATA4)}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushData4BadN(t *testing.T) {
	prog := []byte{byte(PUSHDATA4), 1, 0, 0, 0}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushData4ShortN(t *testing.T) {
	prog := []byte{byte(PUSHDATA4), 0, 0, 0}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPushData4Good(t *testing.T) {
	prog := makeProgram(PUSHDATA4, 3, 0, 0, 0, 1, 2, 3)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte{1, 2, 3}, vm.estack.Pop().Bytes())
}

func TestNOTNoArgument(t *testing.T) {
	prog := makeProgram(NOT)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestNOTBool(t *testing.T) {
	prog := makeProgram(NOT)
	vm := load(prog)
	vm.estack.PushVal(false)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestNOTNonZeroInt(t *testing.T) {
	prog := makeProgram(NOT)
	vm := load(prog)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestNOTArray(t *testing.T) {
	prog := makeProgram(NOT)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestNOTStruct(t *testing.T) {
	prog := makeProgram(NOT)
	vm := load(prog)
	vm.estack.Push(NewElement(&StructItem{[]StackItem{}}))
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestNOTByteArray0(t *testing.T) {
	prog := makeProgram(NOT)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 0})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestNOTByteArray1(t *testing.T) {
	prog := makeProgram(NOT)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 1})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestAdd(t *testing.T) {
	prog := makeProgram(ADD)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(6), vm.estack.Pop().BigInt().Int64())
}

func TestMul(t *testing.T) {
	prog := makeProgram(MUL)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(8), vm.estack.Pop().BigInt().Int64())
}

func TestDiv(t *testing.T) {
	prog := makeProgram(DIV)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(2), vm.estack.Pop().BigInt().Int64())
}

func TestSub(t *testing.T) {
	prog := makeProgram(SUB)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(2), vm.estack.Pop().BigInt().Int64())
}

func TestLT(t *testing.T) {
	prog := makeProgram(LT)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestLTE(t *testing.T) {
	prog := makeProgram(LTE)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestGT(t *testing.T) {
	prog := makeProgram(GT)
	vm := load(prog)
	vm.estack.PushVal(9)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, true, vm.estack.Pop().Bool())

}

func TestGTE(t *testing.T) {
	prog := makeProgram(GTE)
	vm := load(prog)
	vm.estack.PushVal(3)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestDepth(t *testing.T) {
	prog := makeProgram(DEPTH)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(3), vm.estack.Pop().BigInt().Int64())
}

func TestEQUALNoArguments(t *testing.T) {
	prog := makeProgram(EQUAL)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestEQUALBad1Argument(t *testing.T) {
	prog := makeProgram(EQUAL)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestEQUALGoodInteger(t *testing.T) {
	prog := makeProgram(EQUAL)
	vm := load(prog)
	vm.estack.PushVal(5)
	vm.estack.PushVal(5)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestNumEqual(t *testing.T) {
	prog := makeProgram(NUMEQUAL)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestNumNotEqual(t *testing.T) {
	prog := makeProgram(NUMNOTEQUAL)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestINC(t *testing.T) {
	prog := makeProgram(INC)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, big.NewInt(2), vm.estack.Pop().BigInt())
}

func TestNEWARRAYInteger(t *testing.T) {
	prog := makeProgram(NEWARRAY)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{make([]StackItem, 1)}, vm.estack.Pop().value)
}

func TestNEWARRAYStruct(t *testing.T) {
	prog := makeProgram(NEWARRAY)
	vm := load(prog)
	arr := []StackItem{makeStackItem(42)}
	vm.estack.Push(&Element{value: &StructItem{arr}})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{arr}, vm.estack.Pop().value)
}

func TestNEWARRAYArray(t *testing.T) {
	prog := makeProgram(NEWARRAY)
	vm := load(prog)
	arr := []StackItem{makeStackItem(42)}
	vm.estack.Push(&Element{value: &ArrayItem{arr}})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{arr}, vm.estack.Pop().value)
}

func TestNEWARRAYWrongType(t *testing.T) {
	prog := makeProgram(NEWARRAY)
	vm := load(prog)
	vm.estack.Push(NewElement([]byte{}))
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestNEWSTRUCTInteger(t *testing.T) {
	prog := makeProgram(NEWSTRUCT)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{make([]StackItem, 1)}, vm.estack.Pop().value)
}

func TestNEWSTRUCTArray(t *testing.T) {
	prog := makeProgram(NEWSTRUCT)
	vm := load(prog)
	arr := []StackItem{makeStackItem(42)}
	vm.estack.Push(&Element{value: &ArrayItem{arr}})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{arr}, vm.estack.Pop().value)
}

func TestNEWSTRUCTStruct(t *testing.T) {
	prog := makeProgram(NEWSTRUCT)
	vm := load(prog)
	arr := []StackItem{makeStackItem(42)}
	vm.estack.Push(&Element{value: &StructItem{arr}})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{arr}, vm.estack.Pop().value)
}

func TestNEWSTRUCTWrongType(t *testing.T) {
	prog := makeProgram(NEWSTRUCT)
	vm := load(prog)
	vm.estack.Push(NewElement([]byte{}))
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestAPPENDArray(t *testing.T) {
	prog := makeProgram(DUP, PUSH5, APPEND)
	vm := load(prog)
	vm.estack.Push(&Element{value: &ArrayItem{}})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{[]StackItem{makeStackItem(5)}}, vm.estack.Pop().value)
}

func TestAPPENDStruct(t *testing.T) {
	prog := makeProgram(DUP, PUSH5, APPEND)
	vm := load(prog)
	vm.estack.Push(&Element{value: &StructItem{}})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{[]StackItem{makeStackItem(5)}}, vm.estack.Pop().value)
}

func TestAPPENDBadNoArguments(t *testing.T) {
	prog := makeProgram(APPEND)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestAPPENDBad1Argument(t *testing.T) {
	prog := makeProgram(APPEND)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestAPPENDWrongType(t *testing.T) {
	prog := makeProgram(APPEND)
	vm := load(prog)
	vm.estack.PushVal([]byte{})
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSIGNNoArgument(t *testing.T) {
	prog := makeProgram(SIGN)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSIGNWrongType(t *testing.T) {
	prog := makeProgram(SIGN)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSIGNBool(t *testing.T) {
	prog := makeProgram(SIGN)
	vm := load(prog)
	vm.estack.PushVal(false)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(0)}, vm.estack.Pop().value)
}

func TestSIGNPositiveInt(t *testing.T) {
	prog := makeProgram(SIGN)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(1)}, vm.estack.Pop().value)
}

func TestSIGNNegativeInt(t *testing.T) {
	prog := makeProgram(SIGN)
	vm := load(prog)
	vm.estack.PushVal(-1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(-1)}, vm.estack.Pop().value)
}

func TestSIGNZero(t *testing.T) {
	prog := makeProgram(SIGN)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(0)}, vm.estack.Pop().value)
}

func TestSIGNByteArray(t *testing.T) {
	prog := makeProgram(SIGN)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 1})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(1)}, vm.estack.Pop().value)
}

func TestAppCall(t *testing.T) {
	prog := []byte{byte(APPCALL)}
	hash := util.Uint160{}
	prog = append(prog, hash.Bytes()...)
	prog = append(prog, byte(RET))

	vm := load(prog)
	vm.scripts[hash] = makeProgram(DEPTH)
	vm.estack.PushVal(2)

	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	elem := vm.estack.Pop() // depth should be 1
	assert.Equal(t, int64(1), elem.BigInt().Int64())
}

func TestSimpleCall(t *testing.T) {
	progStr := "52c56b525a7c616516006c766b00527ac46203006c766b00c3616c756653c56b6c766b00527ac46c766b51527ac46203006c766b00c36c766b51c393616c7566"
	result := 12

	prog, err := hex.DecodeString(progStr)
	if err != nil {
		t.Fatal(err)
	}
	vm := load(prog)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, result, int(vm.estack.Pop().BigInt().Int64()))
}

func TestNZtrue(t *testing.T) {
	prog := makeProgram(NZ)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestNZfalse(t *testing.T) {
	prog := makeProgram(NZ)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestPICKbadNoitem(t *testing.T) {
	prog := makeProgram(PICK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPICKbadNegative(t *testing.T) {
	prog := makeProgram(PICK)
	vm := load(prog)
	vm.estack.PushVal(-1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPICKgood(t *testing.T) {
	prog := makeProgram(PICK)
	result := 2
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal(1)
	vm.estack.PushVal(result)
	vm.estack.PushVal(3)
	vm.estack.PushVal(4)
	vm.estack.PushVal(5)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(result), vm.estack.Pop().BigInt().Int64())
}

func TestXTUCKbadNoitem(t *testing.T) {
	prog := makeProgram(XTUCK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestXTUCKbadNoN(t *testing.T) {
	prog := makeProgram(XTUCK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestXTUCKbadNegative(t *testing.T) {
	prog := makeProgram(XTUCK)
	vm := load(prog)
	vm.estack.PushVal(-1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestXTUCKbadZero(t *testing.T) {
	prog := makeProgram(XTUCK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(0)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestXTUCKgood(t *testing.T) {
	prog := makeProgram(XTUCK)
	topelement := 5
	xtuckdepth := 3
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.estack.PushVal(4)
	vm.estack.PushVal(topelement)
	vm.estack.PushVal(xtuckdepth)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(topelement), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(topelement), vm.estack.Peek(xtuckdepth).BigInt().Int64())
}

func TestTUCKbadNoitems(t *testing.T) {
	prog := makeProgram(TUCK)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestTUCKbadNoitem(t *testing.T) {
	prog := makeProgram(TUCK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestTUCKgood(t *testing.T) {
	prog := makeProgram(TUCK)
	vm := load(prog)
	vm.estack.PushVal(42)
	vm.estack.PushVal(34)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(34), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(42), vm.estack.Peek(1).BigInt().Int64())
	assert.Equal(t, int64(34), vm.estack.Peek(2).BigInt().Int64())
}

func TestTUCKgood2(t *testing.T) {
	prog := makeProgram(TUCK)
	vm := load(prog)
	vm.estack.PushVal(11)
	vm.estack.PushVal(42)
	vm.estack.PushVal(34)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(34), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(42), vm.estack.Peek(1).BigInt().Int64())
	assert.Equal(t, int64(34), vm.estack.Peek(2).BigInt().Int64())
	assert.Equal(t, int64(11), vm.estack.Peek(3).BigInt().Int64())
}

func TestOVERbadNoitem(t *testing.T) {
	prog := makeProgram(OVER)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestOVERbadNoitems(t *testing.T) {
	prog := makeProgram(OVER)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestOVERgood(t *testing.T) {
	prog := makeProgram(OVER)
	vm := load(prog)
	vm.estack.PushVal(42)
	vm.estack.PushVal(34)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(42), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(34), vm.estack.Peek(1).BigInt().Int64())
	assert.Equal(t, int64(42), vm.estack.Peek(2).BigInt().Int64())
	assert.Equal(t, 3, vm.estack.Len())
}

func TestXDROPbadNoitem(t *testing.T) {
	prog := makeProgram(XDROP)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestXDROPbadNoN(t *testing.T) {
	prog := makeProgram(XDROP)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestXDROPbadNegative(t *testing.T) {
	prog := makeProgram(XDROP)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(-1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestXDROPgood(t *testing.T) {
	prog := makeProgram(XDROP)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, int64(2), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(1), vm.estack.Peek(1).BigInt().Int64())
}

func TestINVERTbadNoitem(t *testing.T) {
	prog := makeProgram(INVERT)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestINVERTgood1(t *testing.T) {
	prog := makeProgram(INVERT)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(-1), vm.estack.Peek(0).BigInt().Int64())
}

func TestINVERTgood2(t *testing.T) {
	prog := makeProgram(INVERT)
	vm := load(prog)
	vm.estack.PushVal(-1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(0), vm.estack.Peek(0).BigInt().Int64())
}

func TestINVERTgood3(t *testing.T) {
	prog := makeProgram(INVERT)
	vm := load(prog)
	vm.estack.PushVal(0x69)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, int64(-0x6A), vm.estack.Peek(0).BigInt().Int64())
}

func TestCATBadNoArgs(t *testing.T) {
	prog := makeProgram(CAT)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestCATBadOneArg(t *testing.T) {
	prog := makeProgram(CAT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abc"))
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestCATGood(t *testing.T) {
	prog := makeProgram(CAT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abc"))
	vm.estack.PushVal([]byte("def"))
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("abcdef"), vm.estack.Peek(0).Bytes())
}

func TestCATInt0ByteArray(t *testing.T) {
	prog := makeProgram(CAT)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal([]byte{})
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ByteArrayItem{[]byte{}}, vm.estack.Pop().value)
}

func TestCATByteArrayInt1(t *testing.T) {
	prog := makeProgram(CAT)
	vm := load(prog)
	vm.estack.PushVal([]byte{})
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ByteArrayItem{[]byte{1}}, vm.estack.Pop().value)
}

func TestSUBSTRBadNoArgs(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSUBSTRBadOneArg(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSUBSTRBadTwoArgs(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSUBSTRGood(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("bc"), vm.estack.Peek(0).Bytes())
}

func TestSUBSTRBadOffset(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(6)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSUBSTRBadLen(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(1)
	vm.estack.PushVal(6)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSUBSTRBad387(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	b := make([]byte, 6, 20)
	copy(b, "abcdef")
	vm.estack.PushVal(b)
	vm.estack.PushVal(1)
	vm.estack.PushVal(6)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSUBSTRBadNegativeOffset(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(-1)
	vm.estack.PushVal(3)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestSUBSTRBadNegativeLen(t *testing.T) {
	prog := makeProgram(SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(3)
	vm.estack.PushVal(-1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestLEFTBadNoArgs(t *testing.T) {
	prog := makeProgram(LEFT)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestLEFTBadNoString(t *testing.T) {
	prog := makeProgram(LEFT)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestLEFTBadNegativeLen(t *testing.T) {
	prog := makeProgram(LEFT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(-1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestLEFTGood(t *testing.T) {
	prog := makeProgram(LEFT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("ab"), vm.estack.Peek(0).Bytes())
}

func TestLEFTGoodLen(t *testing.T) {
	prog := makeProgram(LEFT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(8)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("abcdef"), vm.estack.Peek(0).Bytes())
}

func TestRIGHTBadNoArgs(t *testing.T) {
	prog := makeProgram(RIGHT)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestRIGHTBadNoString(t *testing.T) {
	prog := makeProgram(RIGHT)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestRIGHTBadNegativeLen(t *testing.T) {
	prog := makeProgram(RIGHT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(-1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestRIGHTGood(t *testing.T) {
	prog := makeProgram(RIGHT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("ef"), vm.estack.Peek(0).Bytes())
}

func TestRIGHTBadLen(t *testing.T) {
	prog := makeProgram(RIGHT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(8)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPACKBadLen(t *testing.T) {
	prog := makeProgram(PACK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestPACKGoodZeroLen(t *testing.T) {
	prog := makeProgram(PACK)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []StackItem{}, vm.estack.Peek(0).Array())
}

func TestPACKGood(t *testing.T) {
	prog := makeProgram(PACK)
	elements := []int{55, 34, 42}
	vm := load(prog)
	// canary
	vm.estack.PushVal(1)
	for i := len(elements) - 1; i >= 0; i-- {
		vm.estack.PushVal(elements[i])
	}
	vm.estack.PushVal(len(elements))
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 2, vm.estack.Len())
	a := vm.estack.Peek(0).Array()
	assert.Equal(t, len(elements), len(a))
	for i := 0; i < len(elements); i++ {
		e := a[i].Value().(*big.Int)
		assert.Equal(t, int64(elements[i]), e.Int64())
	}
	assert.Equal(t, int64(1), vm.estack.Peek(1).BigInt().Int64())
}

func TestUNPACKBadNotArray(t *testing.T) {
	prog := makeProgram(UNPACK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestUNPACKGood(t *testing.T) {
	prog := makeProgram(UNPACK)
	elements := []int{55, 34, 42}
	vm := load(prog)
	// canary
	vm.estack.PushVal(1)
	vm.estack.PushVal(elements)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 5, vm.estack.Len())
	assert.Equal(t, int64(len(elements)), vm.estack.Peek(0).BigInt().Int64())
	for k, v := range elements {
		assert.Equal(t, int64(v), vm.estack.Peek(k+1).BigInt().Int64())
	}
	assert.Equal(t, int64(1), vm.estack.Peek(len(elements)+1).BigInt().Int64())
}

func TestREVERSEBadNotArray(t *testing.T) {
	prog := makeProgram(REVERSE)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestREVERSEGoodOneElem(t *testing.T) {
	prog := makeProgram(REVERSE)
	elements := []int{22}
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(elements)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 2, vm.estack.Len())
	a := vm.estack.Peek(0).Array()
	assert.Equal(t, len(elements), len(a))
	e := a[0].Value().(*big.Int)
	assert.Equal(t, int64(elements[0]), e.Int64())
}

func TestREVERSEGood(t *testing.T) {
	eodd := []int{22, 34, 42, 55, 81}
	even := []int{22, 34, 42, 55, 81, 99}
	eall := [][]int{eodd, even}

	for _, elements := range eall {
		prog := makeProgram(REVERSE)
		vm := load(prog)
		vm.estack.PushVal(1)
		vm.estack.PushVal(elements)
		vm.Run()
		assert.Equal(t, false, vm.state.HasFlag(faultState))
		assert.Equal(t, 2, vm.estack.Len())
		a := vm.estack.Peek(0).Array()
		assert.Equal(t, len(elements), len(a))
		for k, v := range elements {
			e := a[len(a)-1-k].Value().(*big.Int)
			assert.Equal(t, int64(v), e.Int64())
		}
		assert.Equal(t, int64(1), vm.estack.Peek(1).BigInt().Int64())
	}
}

func TestREMOVEBadNoArgs(t *testing.T) {
	prog := makeProgram(REMOVE)
	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestREMOVEBadOneArg(t *testing.T) {
	prog := makeProgram(REMOVE)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestREMOVEBadNotArray(t *testing.T) {
	prog := makeProgram(REMOVE)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(1)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestREMOVEBadIndex(t *testing.T) {
	prog := makeProgram(REMOVE)
	elements := []int{22, 34, 42, 55, 81}
	vm := load(prog)
	vm.estack.PushVal(elements)
	vm.estack.PushVal(10)
	vm.Run()
	assert.Equal(t, true, vm.state.HasFlag(faultState))
}

func TestREMOVEGood(t *testing.T) {
	prog := makeProgram(REMOVE)
	elements := []int{22, 34, 42, 55, 81}
	reselements := []int{22, 34, 55, 81}
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(elements)
	vm.estack.PushVal(2)
	vm.Run()
	assert.Equal(t, false, vm.state.HasFlag(faultState))
	assert.Equal(t, 2, vm.estack.Len())
	a := vm.estack.Peek(0).Array()
	assert.Equal(t, len(reselements), len(a))
	for k, v := range reselements {
		e := a[k].Value().(*big.Int)
		assert.Equal(t, int64(v), e.Int64())
	}
	assert.Equal(t, int64(1), vm.estack.Peek(1).BigInt().Int64())
}

func makeProgram(opcodes ...Instruction) []byte {
	prog := make([]byte, len(opcodes)+1) // RET
	for i := 0; i < len(opcodes); i++ {
		prog[i] = byte(opcodes[i])
	}
	prog[len(prog)-1] = byte(RET)
	return prog
}

func load(prog []byte) *VM {
	vm := New(ModeMute)
	vm.mute = true
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
