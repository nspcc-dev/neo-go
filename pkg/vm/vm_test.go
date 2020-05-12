package vm

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fooInteropGetter(id uint32) *InteropFuncPrice {
	if id == emit.InteropNameToID([]byte("foo")) {
		return &InteropFuncPrice{func(evm *VM) error {
			evm.Estack().PushVal(1)
			return nil
		}, 1}
	}
	return nil
}

func TestInteropHook(t *testing.T) {
	v := New()
	v.RegisterInteropGetter(fooInteropGetter)

	buf := io.NewBufBinWriter()
	emit.Syscall(buf.BinWriter, "foo")
	emit.Opcode(buf.BinWriter, opcode.RET)
	v.Load(buf.Bytes())
	runVM(t, v)
	assert.Equal(t, 1, v.estack.Len())
	assert.Equal(t, big.NewInt(1), v.estack.Pop().value.Value())
}

func TestRegisterInteropGetter(t *testing.T) {
	v := New()
	currRegistered := len(v.getInterop)
	v.RegisterInteropGetter(fooInteropGetter)
	assert.Equal(t, currRegistered+1, len(v.getInterop))
}

func TestVM_SetPriceGetter(t *testing.T) {
	v := New()
	prog := []byte{
		byte(opcode.PUSH4), byte(opcode.PUSH2),
		byte(opcode.PUSHDATA1), 0x01, 0x01,
		byte(opcode.PUSHDATA1), 0x02, 0xCA, 0xFE,
		byte(opcode.PUSH4), byte(opcode.RET),
	}

	t.Run("no price getter", func(t *testing.T) {
		v.Load(prog)
		runVM(t, v)

		require.EqualValues(t, 0, v.GasConsumed())
	})

	v.SetPriceGetter(func(_ *VM, op opcode.Opcode, p []byte) util.Fixed8 {
		if op == opcode.PUSH4 {
			return 1
		} else if op == opcode.PUSHDATA1 && bytes.Equal(p, []byte{0xCA, 0xFE}) {
			return 7
		}

		return 0
	})

	t.Run("with price getter", func(t *testing.T) {
		v.Load(prog)
		runVM(t, v)

		require.EqualValues(t, 9, v.GasConsumed())
	})

	t.Run("with sufficient gas limit", func(t *testing.T) {
		v.Load(prog)
		v.SetGasLimit(9)
		runVM(t, v)

		require.EqualValues(t, 9, v.GasConsumed())
	})

	t.Run("with small gas limit", func(t *testing.T) {
		v.Load(prog)
		v.SetGasLimit(8)
		checkVMFailed(t, v)
	})
}

func TestBytesToPublicKey(t *testing.T) {
	v := New()
	cache := v.GetPublicKeys()
	assert.Equal(t, 0, len(cache))
	keyHex := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	keyBytes, _ := hex.DecodeString(keyHex)
	key := v.bytesToPublicKey(keyBytes)
	assert.NotNil(t, key)
	key2 := v.bytesToPublicKey(keyBytes)
	assert.Equal(t, key, key2)

	cache = v.GetPublicKeys()
	assert.Equal(t, 1, len(cache))
	assert.NotNil(t, cache[string(keyBytes)])

	keyBytes[0] = 0xff
	require.Panics(t, func() { v.bytesToPublicKey(keyBytes) })
}

func TestPushBytes1to75(t *testing.T) {
	buf := io.NewBufBinWriter()
	for i := 1; i <= 75; i++ {
		b := randomBytes(i)
		emit.Bytes(buf.BinWriter, b)
		vm := load(buf.Bytes())
		err := vm.Step()
		require.NoError(t, err)

		assert.Equal(t, 1, vm.estack.Len())

		elem := vm.estack.Pop()
		assert.IsType(t, &ByteArrayItem{}, elem.value)
		assert.IsType(t, elem.Bytes(), b)
		assert.Equal(t, 0, vm.estack.Len())

		errExec := vm.execute(nil, opcode.RET, nil)
		require.NoError(t, errExec)

		assert.Equal(t, 0, vm.astack.Len())
		assert.Equal(t, 0, vm.istack.Len())
		buf.Reset()
	}
}

func runVM(t *testing.T, vm *VM) {
	err := vm.Run()
	require.NoError(t, err)
	assert.Equal(t, false, vm.HasFailed())
}

func checkVMFailed(t *testing.T, vm *VM) {
	err := vm.Run()
	require.Error(t, err)
	assert.Equal(t, true, vm.HasFailed())
}

func TestStackLimitPUSH1Good(t *testing.T) {
	prog := make([]byte, MaxStackSize*2)
	for i := 0; i < MaxStackSize; i++ {
		prog[i] = byte(opcode.PUSH1)
	}
	for i := MaxStackSize; i < MaxStackSize*2; i++ {
		prog[i] = byte(opcode.DROP)
	}

	v := load(prog)
	runVM(t, v)
}

func TestStackLimitPUSH1Bad(t *testing.T) {
	prog := make([]byte, MaxStackSize+1)
	for i := range prog {
		prog[i] = byte(opcode.PUSH1)
	}
	v := load(prog)
	checkVMFailed(t, v)
}

func TestPUSHINT(t *testing.T) {
	for i := byte(0); i < 5; i++ {
		op := opcode.PUSHINT8 + opcode.Opcode(i)
		t.Run(op.String(), func(t *testing.T) {
			buf := random.Bytes((8 << i) / 8)
			prog := append([]byte{byte(op)}, buf...)
			runWithArgs(t, prog, emit.BytesToInt(buf))
		})
	}
}

func TestPUSHNULL(t *testing.T) {
	prog := makeProgram(opcode.PUSHNULL, opcode.PUSHNULL, opcode.EQUAL)
	v := load(prog)
	require.NoError(t, v.Step())
	require.Equal(t, 1, v.estack.Len())
	runVM(t, v)
	require.True(t, v.estack.Pop().Bool())
}

func TestISNULL(t *testing.T) {
	prog := makeProgram(opcode.ISNULL)
	t.Run("Integer", getTestFuncForVM(prog, false, 1))
	t.Run("Null", getTestFuncForVM(prog, true, NullItem{}))
}

func testISTYPE(t *testing.T, result bool, typ StackItemType, item StackItem) {
	prog := []byte{byte(opcode.ISTYPE), byte(typ)}
	runWithArgs(t, prog, result, item)
}

func TestISTYPE(t *testing.T) {
	t.Run("Integer", func(t *testing.T) {
		testISTYPE(t, true, IntegerT, NewBigIntegerItem(big.NewInt(42)))
		testISTYPE(t, false, IntegerT, NewByteArrayItem([]byte{}))
	})
	t.Run("Boolean", func(t *testing.T) {
		testISTYPE(t, true, BooleanT, NewBoolItem(true))
		testISTYPE(t, false, BooleanT, NewByteArrayItem([]byte{}))
	})
	t.Run("ByteArray", func(t *testing.T) {
		testISTYPE(t, true, ByteArrayT, NewByteArrayItem([]byte{}))
		testISTYPE(t, false, ByteArrayT, NewBigIntegerItem(big.NewInt(42)))
	})
	t.Run("Array", func(t *testing.T) {
		testISTYPE(t, true, ArrayT, NewArrayItem([]StackItem{}))
		testISTYPE(t, false, ArrayT, NewByteArrayItem([]byte{}))
	})
	t.Run("Struct", func(t *testing.T) {
		testISTYPE(t, true, StructT, NewStructItem([]StackItem{}))
		testISTYPE(t, false, StructT, NewByteArrayItem([]byte{}))
	})
	t.Run("Map", func(t *testing.T) {
		testISTYPE(t, true, MapT, NewMapItem())
		testISTYPE(t, false, MapT, NewByteArrayItem([]byte{}))
	})
	t.Run("Interop", func(t *testing.T) {
		testISTYPE(t, true, InteropT, NewInteropItem(42))
		testISTYPE(t, false, InteropT, NewByteArrayItem([]byte{}))
	})
}

func testCONVERT(to StackItemType, item, res StackItem) func(t *testing.T) {
	return func(t *testing.T) {
		prog := []byte{byte(opcode.CONVERT), byte(to)}
		runWithArgs(t, prog, res, item)
	}
}

func TestCONVERT(t *testing.T) {
	type convertTC struct {
		item, res StackItem
	}
	arr := []StackItem{
		NewBigIntegerItem(big.NewInt(7)),
		NewByteArrayItem([]byte{4, 8, 15}),
	}
	m := NewMapItem()
	m.Add(NewByteArrayItem([]byte{1}), NewByteArrayItem([]byte{2}))

	getName := func(item StackItem, typ StackItemType) string { return fmt.Sprintf("%s->%s", item, typ) }

	t.Run("->Bool", func(t *testing.T) {
		testBool := func(a, b StackItem) func(t *testing.T) {
			return testCONVERT(BooleanT, a, b)
		}

		trueCases := []StackItem{
			NewBoolItem(true), NewBigIntegerItem(big.NewInt(11)), NewByteArrayItem([]byte{1, 2, 3}),
			NewArrayItem(arr), NewArrayItem(nil),
			NewStructItem(arr), NewStructItem(nil),
			NewMapItem(), m, NewInteropItem(struct{}{}),
			NewPointerItem(0, []byte{}),
		}
		for i := range trueCases {
			t.Run(getName(trueCases[i], BooleanT), testBool(trueCases[i], NewBoolItem(true)))
		}

		falseCases := []StackItem{
			NewBigIntegerItem(big.NewInt(0)), NewByteArrayItem([]byte{0, 0}), NewBoolItem(false),
		}
		for i := range falseCases {
			testBool(falseCases[i], NewBoolItem(false))
		}
	})

	t.Run("compound/interop -> basic", func(t *testing.T) {
		types := []StackItemType{IntegerT, ByteArrayT}
		items := []StackItem{NewArrayItem(nil), NewStructItem(nil), NewMapItem(), NewInteropItem(struct{}{})}
		for _, typ := range types {
			for j := range items {
				t.Run(getName(items[j], typ), testCONVERT(typ, items[j], nil))
			}
		}
	})

	t.Run("primitive -> Integer/ByteArray", func(t *testing.T) {
		n := big.NewInt(42)
		b := emit.IntToBytes(n)

		itemInt := NewBigIntegerItem(n)
		itemBytes := NewByteArrayItem(b)

		trueCases := map[StackItemType][]convertTC{
			IntegerT: {
				{itemInt, itemInt},
				{itemBytes, itemInt},
				{NewBoolItem(true), NewBigIntegerItem(big.NewInt(1))},
				{NewBoolItem(false), NewBigIntegerItem(big.NewInt(0))},
			},
			ByteArrayT: {
				{itemInt, itemBytes},
				{itemBytes, itemBytes},
				{NewBoolItem(true), NewByteArrayItem([]byte{1})},
				{NewBoolItem(false), NewByteArrayItem([]byte{0})},
			},
		}

		for typ := range trueCases {
			for _, tc := range trueCases[typ] {
				t.Run(getName(tc.item, typ), testCONVERT(typ, tc.item, tc.res))
			}
		}
	})

	t.Run("Struct<->Array", func(t *testing.T) {
		arrayItem := NewArrayItem(arr)
		structItem := NewStructItem(arr)
		t.Run("Array->Array", testCONVERT(ArrayT, arrayItem, arrayItem))
		t.Run("Array->Struct", testCONVERT(StructT, arrayItem, structItem))
		t.Run("Struct->Array", testCONVERT(ArrayT, structItem, arrayItem))
		t.Run("Struct->Struct", testCONVERT(StructT, structItem, structItem))
	})

	t.Run("Map->Map", testCONVERT(MapT, m, m))

	ptr := NewPointerItem(1, []byte{1})
	t.Run("Pointer->Pointer", testCONVERT(PointerT, ptr, ptr))

	t.Run("Null->", func(t *testing.T) {
		types := []StackItemType{
			BooleanT, ByteArrayT, IntegerT, ArrayT, StructT, MapT, InteropT, PointerT,
		}
		for i := range types {
			t.Run(types[i].String(), testCONVERT(types[i], NullItem{}, NullItem{}))
		}
	})

	t.Run("->Any", func(t *testing.T) {
		items := []StackItem{
			NewBigIntegerItem(big.NewInt(1)), NewByteArrayItem([]byte{1}), NewBoolItem(true),
			NewArrayItem(arr), NewStructItem(arr), m, NewInteropItem(struct{}{}),
		}

		for i := range items {
			t.Run(items[i].String(), testCONVERT(AnyT, items[i], nil))
		}
	})
}

// appendBigStruct returns a program which:
// 1. pushes size Structs on stack
// 2. packs them into a new struct
// 3. appends them to a zero-length array
// Resulting stack size consists of:
// - struct (size+1)
// - array (1) of struct (size+1)
// which equals to size*2+3 elements in total.
func appendBigStruct(size uint16) []opcode.Opcode {
	prog := make([]opcode.Opcode, size*2)
	for i := uint16(0); i < size; i++ {
		prog[i*2] = opcode.PUSH0
		prog[i*2+1] = opcode.NEWSTRUCT
	}

	return append(prog,
		opcode.PUSHINT16, opcode.Opcode(size), opcode.Opcode(size>>8), // LE
		opcode.PACK, opcode.NEWSTRUCT,
		opcode.DUP,
		opcode.PUSH0, opcode.NEWARRAY, opcode.TOALTSTACK, opcode.DUPFROMALTSTACK,
		opcode.SWAP,
		opcode.APPEND, opcode.RET)
}

func TestStackLimitAPPENDStructGood(t *testing.T) {
	prog := makeProgram(appendBigStruct(MaxStackSize/2 - 2)...)
	v := load(prog)
	runVM(t, v) // size = 2047 = (Max/2-2)*2+3 = Max-1
}

func TestStackLimitAPPENDStructBad(t *testing.T) {
	prog := makeProgram(appendBigStruct(MaxStackSize/2 - 1)...)
	v := load(prog)
	checkVMFailed(t, v) // size = 2049 = (Max/2-1)*2+3 = Max+1
}

func TestStackLimit(t *testing.T) {
	expected := []struct {
		inst opcode.Opcode
		size int
	}{
		{opcode.PUSH2, 1},
		{opcode.NEWARRAY, 3}, // array + 2 items
		{opcode.TOALTSTACK, 3},
		{opcode.DUPFROMALTSTACK, 4},
		{opcode.NEWSTRUCT, 6}, // all items are copied
		{opcode.NEWMAP, 7},
		{opcode.DUP, 8},
		{opcode.PUSH2, 9},
		{opcode.DUPFROMALTSTACK, 10},
		{opcode.SETITEM, 8}, // -3 items and 1 new element in map
		{opcode.DUP, 9},
		{opcode.PUSH2, 10},
		{opcode.DUPFROMALTSTACK, 11},
		{opcode.SETITEM, 8}, // -3 items and no new elements in map
		{opcode.DUP, 9},
		{opcode.PUSH2, 10},
		{opcode.REMOVE, 7}, // as we have right after NEWMAP
		{opcode.DROP, 6},   // DROP map with no elements
	}

	prog := make([]opcode.Opcode, len(expected))
	for i := range expected {
		prog[i] = expected[i].inst
	}

	vm := load(makeProgram(prog...))
	for i := range expected {
		require.NoError(t, vm.Step())
		require.Equal(t, expected[i].size, vm.refs.size)
	}
}

func TestPushm1to16(t *testing.T) {
	var prog []byte
	for i := int(opcode.PUSHM1); i <= int(opcode.PUSH16); i++ {
		if i == 80 {
			continue // opcode layout we got here.
		}
		prog = append(prog, byte(i))
	}

	vm := load(prog)
	for i := int(opcode.PUSHM1); i <= int(opcode.PUSH16); i++ {
		err := vm.Step()
		require.NoError(t, err)

		elem := vm.estack.Pop()
		val := i - int(opcode.PUSH1) + 1
		assert.Equal(t, elem.BigInt().Int64(), int64(val))
	}
}

func TestPUSHDATA1(t *testing.T) {
	t.Run("Good", getTestFuncForVM([]byte{byte(opcode.PUSHDATA1), 3, 1, 2, 3}, []byte{1, 2, 3}))
	t.Run("NoN", getTestFuncForVM([]byte{byte(opcode.PUSHDATA1)}, nil))
	t.Run("BadN", getTestFuncForVM([]byte{byte(opcode.PUSHDATA1), 1}, nil))
}

func TestPUSHDATA2(t *testing.T) {
	t.Run("Good", getTestFuncForVM([]byte{byte(opcode.PUSHDATA2), 3, 0, 1, 2, 3}, []byte{1, 2, 3}))
	t.Run("NoN", getTestFuncForVM([]byte{byte(opcode.PUSHDATA2)}, nil))
	t.Run("ShortN", getTestFuncForVM([]byte{byte(opcode.PUSHDATA2), 0}, nil))
	t.Run("BadN", getTestFuncForVM([]byte{byte(opcode.PUSHDATA2), 1, 0}, nil))
}

func TestPUSHDATA4(t *testing.T) {
	t.Run("Good", getTestFuncForVM([]byte{byte(opcode.PUSHDATA4), 3, 0, 0, 0, 1, 2, 3}, []byte{1, 2, 3}))
	t.Run("NoN", getTestFuncForVM([]byte{byte(opcode.PUSHDATA4)}, nil))
	t.Run("BadN", getTestFuncForVM([]byte{byte(opcode.PUSHDATA4), 1, 0, 0, 0}, nil))
	t.Run("ShortN", getTestFuncForVM([]byte{byte(opcode.PUSHDATA4), 0, 0, 0}, nil))
}

func TestPushData4BigN(t *testing.T) {
	prog := make([]byte, 1+4+MaxItemSize+1)
	prog[0] = byte(opcode.PUSHDATA4)
	binary.LittleEndian.PutUint32(prog[1:], MaxItemSize+1)

	vm := load(prog)
	checkVMFailed(t, vm)
}

func getEnumeratorProg(n int, isIter bool) (prog []byte) {
	prog = append(prog, byte(opcode.TOALTSTACK))
	for i := 0; i < n; i++ {
		prog = append(prog, byte(opcode.DUPFROMALTSTACK))
		prog = append(prog, getSyscallProg("Neo.Enumerator.Next")...)
		prog = append(prog, byte(opcode.DUPFROMALTSTACK))
		prog = append(prog, getSyscallProg("Neo.Enumerator.Value")...)
		if isIter {
			prog = append(prog, byte(opcode.DUPFROMALTSTACK))
			prog = append(prog, getSyscallProg("Neo.Iterator.Key")...)
		}
	}
	prog = append(prog, byte(opcode.DUPFROMALTSTACK))
	prog = append(prog, getSyscallProg("Neo.Enumerator.Next")...)

	return
}

func checkEnumeratorStack(t *testing.T, vm *VM, arr []StackItem) {
	require.Equal(t, len(arr)+1, vm.estack.Len())
	require.Equal(t, NewBoolItem(false), vm.estack.Peek(0).value)
	for i := 0; i < len(arr); i++ {
		require.Equal(t, arr[i], vm.estack.Peek(i+1).value, "pos: %d", i+1)
	}
}

func testIterableCreate(t *testing.T, typ string) {
	isIter := typ == "Iterator"
	prog := getSyscallProg("Neo." + typ + ".Create")
	prog = append(prog, getEnumeratorProg(2, isIter)...)

	vm := load(prog)
	arr := []StackItem{
		NewBigIntegerItem(big.NewInt(42)),
		NewByteArrayItem([]byte{3, 2, 1}),
	}
	vm.estack.Push(&Element{value: NewArrayItem(arr)})

	runVM(t, vm)
	if isIter {
		checkEnumeratorStack(t, vm, []StackItem{
			makeStackItem(1), arr[1], NewBoolItem(true),
			makeStackItem(0), arr[0], NewBoolItem(true),
		})
	} else {
		checkEnumeratorStack(t, vm, []StackItem{
			arr[1], NewBoolItem(true),
			arr[0], NewBoolItem(true),
		})
	}
}

func TestEnumeratorCreate(t *testing.T) {
	testIterableCreate(t, "Enumerator")
}

func TestIteratorCreate(t *testing.T) {
	testIterableCreate(t, "Iterator")
}

func testIterableConcat(t *testing.T, typ string) {
	isIter := typ == "Iterator"
	prog := getSyscallProg("Neo." + typ + ".Create")
	prog = append(prog, byte(opcode.SWAP))
	prog = append(prog, getSyscallProg("Neo."+typ+".Create")...)
	prog = append(prog, getSyscallProg("Neo."+typ+".Concat")...)
	prog = append(prog, getEnumeratorProg(3, isIter)...)
	vm := load(prog)

	arr := []StackItem{
		NewBoolItem(false),
		NewBigIntegerItem(big.NewInt(123)),
		NewMapItem(),
	}
	vm.estack.Push(&Element{value: NewArrayItem(arr[:1])})
	vm.estack.Push(&Element{value: NewArrayItem(arr[1:])})

	runVM(t, vm)

	if isIter {
		// Yes, this is how iterators are concatenated in reference VM
		// https://github.com/neo-project/neo/blob/master-2.x/neo.UnitTests/UT_ConcatenatedIterator.cs#L54
		checkEnumeratorStack(t, vm, []StackItem{
			makeStackItem(1), arr[2], NewBoolItem(true),
			makeStackItem(0), arr[1], NewBoolItem(true),
			makeStackItem(0), arr[0], NewBoolItem(true),
		})
	} else {
		checkEnumeratorStack(t, vm, []StackItem{
			arr[2], NewBoolItem(true),
			arr[1], NewBoolItem(true),
			arr[0], NewBoolItem(true),
		})
	}
}

func TestEnumeratorConcat(t *testing.T) {
	testIterableConcat(t, "Enumerator")
}

func TestIteratorConcat(t *testing.T) {
	testIterableConcat(t, "Iterator")
}

func TestIteratorKeys(t *testing.T) {
	prog := getSyscallProg("Neo.Iterator.Create")
	prog = append(prog, getSyscallProg("Neo.Iterator.Keys")...)
	prog = append(prog, byte(opcode.TOALTSTACK), byte(opcode.DUPFROMALTSTACK))
	prog = append(prog, getEnumeratorProg(2, false)...)

	v := load(prog)
	arr := NewArrayItem([]StackItem{
		NewBoolItem(false),
		NewBigIntegerItem(big.NewInt(42)),
	})
	v.estack.PushVal(arr)

	runVM(t, v)

	checkEnumeratorStack(t, v, []StackItem{
		NewBigIntegerItem(big.NewInt(1)), NewBoolItem(true),
		NewBigIntegerItem(big.NewInt(0)), NewBoolItem(true),
	})
}

func TestIteratorValues(t *testing.T) {
	prog := getSyscallProg("Neo.Iterator.Create")
	prog = append(prog, getSyscallProg("Neo.Iterator.Values")...)
	prog = append(prog, byte(opcode.TOALTSTACK), byte(opcode.DUPFROMALTSTACK))
	prog = append(prog, getEnumeratorProg(2, false)...)

	v := load(prog)
	m := NewMapItem()
	m.Add(NewBigIntegerItem(big.NewInt(1)), NewBoolItem(false))
	m.Add(NewByteArrayItem([]byte{32}), NewByteArrayItem([]byte{7}))
	v.estack.PushVal(m)

	runVM(t, v)
	require.Equal(t, 5, v.estack.Len())
	require.Equal(t, NewBoolItem(false), v.estack.Peek(0).value)

	// Map values can be enumerated in any order.
	i1, i2 := 1, 3
	if _, ok := v.estack.Peek(i1).value.(*BoolItem); !ok {
		i1, i2 = i2, i1
	}

	require.Equal(t, NewBoolItem(false), v.estack.Peek(i1).value)
	require.Equal(t, NewByteArrayItem([]byte{7}), v.estack.Peek(i2).value)

	require.Equal(t, NewBoolItem(true), v.estack.Peek(2).value)
	require.Equal(t, NewBoolItem(true), v.estack.Peek(4).value)
}

func getSyscallProg(name string) (prog []byte) {
	buf := io.NewBufBinWriter()
	emit.Syscall(buf.BinWriter, name)
	return buf.Bytes()
}

func getSerializeProg() (prog []byte) {
	prog = append(prog, getSyscallProg("Neo.Runtime.Serialize")...)
	prog = append(prog, getSyscallProg("Neo.Runtime.Deserialize")...)
	prog = append(prog, byte(opcode.RET))

	return
}

func testSerialize(t *testing.T, vm *VM) {
	err := vm.Step()
	require.NoError(t, err)
	require.Equal(t, 1, vm.estack.Len())
	require.IsType(t, (*ByteArrayItem)(nil), vm.estack.Top().value)

	err = vm.Step()
	require.NoError(t, err)
	require.Equal(t, 1, vm.estack.Len())
}

func TestSerializeBool(t *testing.T) {
	vm := load(getSerializeProg())
	vm.estack.PushVal(true)

	testSerialize(t, vm)

	require.IsType(t, (*BoolItem)(nil), vm.estack.Top().value)
	require.Equal(t, true, vm.estack.Top().Bool())
}

func TestSerializeByteArray(t *testing.T) {
	vm := load(getSerializeProg())
	value := []byte{1, 2, 3}
	vm.estack.PushVal(value)

	testSerialize(t, vm)

	require.IsType(t, (*ByteArrayItem)(nil), vm.estack.Top().value)
	require.Equal(t, value, vm.estack.Top().Bytes())
}

func TestSerializeInteger(t *testing.T) {
	vm := load(getSerializeProg())
	value := int64(123)
	vm.estack.PushVal(value)

	testSerialize(t, vm)

	require.IsType(t, (*BigIntegerItem)(nil), vm.estack.Top().value)
	require.Equal(t, value, vm.estack.Top().BigInt().Int64())
}

func TestSerializeArray(t *testing.T) {
	vm := load(getSerializeProg())
	item := NewArrayItem([]StackItem{
		makeStackItem(true),
		makeStackItem(123),
		NewMapItem(),
	})

	vm.estack.Push(&Element{value: item})

	testSerialize(t, vm)

	require.IsType(t, (*ArrayItem)(nil), vm.estack.Top().value)
	require.Equal(t, item.value, vm.estack.Top().Array())
}

func TestSerializeArrayBad(t *testing.T) {
	vm := load(getSerializeProg())
	item := NewArrayItem(makeArrayOfType(2, BooleanT))
	item.value[1] = item

	vm.estack.Push(&Element{value: item})

	err := vm.Step()
	require.Error(t, err)
	require.True(t, vm.HasFailed())
}

func TestSerializeDupInteger(t *testing.T) {
	prog := []byte{
		byte(opcode.PUSH0), byte(opcode.NEWARRAY),
		byte(opcode.DUP), byte(opcode.PUSH2), byte(opcode.DUP), byte(opcode.TOALTSTACK), byte(opcode.APPEND),
		byte(opcode.DUP), byte(opcode.FROMALTSTACK), byte(opcode.APPEND),
	}
	vm := load(append(prog, getSerializeProg()...))

	runVM(t, vm)
}

func TestSerializeStruct(t *testing.T) {
	vm := load(getSerializeProg())
	item := NewStructItem([]StackItem{
		makeStackItem(true),
		makeStackItem(123),
		NewMapItem(),
	})

	vm.estack.Push(&Element{value: item})

	testSerialize(t, vm)

	require.IsType(t, (*StructItem)(nil), vm.estack.Top().value)
	require.Equal(t, item.value, vm.estack.Top().Array())
}

func TestDeserializeUnknown(t *testing.T) {
	prog := append(getSyscallProg("Neo.Runtime.Deserialize"), byte(opcode.RET))

	data, err := SerializeItem(NewBigIntegerItem(big.NewInt(123)))
	require.NoError(t, err)

	data[0] = 0xFF

	runWithArgs(t, prog, nil, data)
}

func TestSerializeMap(t *testing.T) {
	vm := load(getSerializeProg())
	item := NewMapItem()
	item.Add(makeStackItem(true), makeStackItem([]byte{1, 2, 3}))
	item.Add(makeStackItem([]byte{0}), makeStackItem(false))

	vm.estack.Push(&Element{value: item})

	testSerialize(t, vm)

	require.IsType(t, (*MapItem)(nil), vm.estack.Top().value)
	require.Equal(t, item.value, vm.estack.Top().value.(*MapItem).value)
}

func TestSerializeMapCompat(t *testing.T) {
	resHex := "480128036b6579280576616c7565"
	res, err := hex.DecodeString(resHex)
	require.NoError(t, err)

	// Create a map, push key and value, add KV to map, serialize.
	buf := io.NewBufBinWriter()
	emit.Opcode(buf.BinWriter, opcode.NEWMAP)
	emit.Opcode(buf.BinWriter, opcode.DUP)
	emit.Bytes(buf.BinWriter, []byte("key"))
	emit.Bytes(buf.BinWriter, []byte("value"))
	emit.Opcode(buf.BinWriter, opcode.SETITEM)
	emit.Syscall(buf.BinWriter, "Neo.Runtime.Serialize")
	require.NoError(t, buf.Err)

	vm := load(buf.Bytes())
	runVM(t, vm)
	assert.Equal(t, res, vm.estack.Pop().Bytes())
}

func TestSerializeInterop(t *testing.T) {
	vm := load(getSerializeProg())
	item := NewInteropItem("kek")

	vm.estack.Push(&Element{value: item})

	err := vm.Step()
	require.Error(t, err)
	require.True(t, vm.HasFailed())
}

func callNTimes(n uint16) []byte {
	return makeProgram(
		opcode.PUSHINT16, opcode.Opcode(n), opcode.Opcode(n>>8), // little-endian
		opcode.TOALTSTACK, opcode.DUPFROMALTSTACK,
		opcode.JMPIF, 0x3, opcode.RET,
		opcode.FROMALTSTACK, opcode.DEC,
		opcode.CALL, 0xF9) // -7 -> JMP to TOALTSTACK)
}

func TestInvocationLimitGood(t *testing.T) {
	prog := callNTimes(MaxInvocationStackSize - 1)
	v := load(prog)
	runVM(t, v)
}

func TestInvocationLimitBad(t *testing.T) {
	prog := callNTimes(MaxInvocationStackSize)
	v := load(prog)
	checkVMFailed(t, v)
}

func isLongJMP(op opcode.Opcode) bool {
	return op == opcode.JMPL || op == opcode.JMPIFL || op == opcode.JMPIFNOTL ||
		op == opcode.JMPEQL || op == opcode.JMPNEL ||
		op == opcode.JMPGEL || op == opcode.JMPGTL ||
		op == opcode.JMPLEL || op == opcode.JMPLTL
}

func getJMPProgram(op opcode.Opcode) []byte {
	prog := []byte{byte(op)}
	if isLongJMP(op) {
		prog = append(prog, 0x07, 0x00, 0x00, 0x00)
	} else {
		prog = append(prog, 0x04)
	}
	return append(prog, byte(opcode.PUSH1), byte(opcode.RET), byte(opcode.PUSH2), byte(opcode.RET))
}

func testJMP(t *testing.T, op opcode.Opcode, res interface{}, items ...interface{}) {
	prog := getJMPProgram(op)
	v := load(prog)
	for i := range items {
		v.estack.PushVal(items[i])
	}
	if res == nil {
		checkVMFailed(t, v)
		return
	}
	runVM(t, v)
	require.EqualValues(t, res, v.estack.Pop().BigInt().Int64())
}

func TestJMPs(t *testing.T) {
	testCases := []struct {
		name  string
		items []interface{}
	}{
		{
			name: "no condition",
		},
		{
			name:  "single item (true)",
			items: []interface{}{true},
		},
		{
			name:  "single item (false)",
			items: []interface{}{false},
		},
		{
			name:  "24 and 42",
			items: []interface{}{24, 42},
		},
		{
			name:  "42 and 24",
			items: []interface{}{42, 24},
		},
		{
			name:  "42 and 42",
			items: []interface{}{42, 42},
		},
	}

	// 2 is true, 1 is false
	results := map[opcode.Opcode][]interface{}{
		opcode.JMP:      {2, 2, 2, 2, 2, 2},
		opcode.JMPIF:    {nil, 2, 1, 2, 2, 2},
		opcode.JMPIFNOT: {nil, 1, 2, 1, 1, 1},
		opcode.JMPEQ:    {nil, nil, nil, 1, 1, 2},
		opcode.JMPNE:    {nil, nil, nil, 2, 2, 1},
		opcode.JMPGE:    {nil, nil, nil, 1, 2, 2},
		opcode.JMPGT:    {nil, nil, nil, 1, 2, 1},
		opcode.JMPLE:    {nil, nil, nil, 2, 1, 2},
		opcode.JMPLT:    {nil, nil, nil, 2, 1, 1},
	}

	for i, tc := range testCases {
		i := i
		t.Run(tc.name, func(t *testing.T) {
			for op := opcode.JMP; op < opcode.JMPLEL; op++ {
				resOp := op
				if isLongJMP(op) {
					resOp--
				}
				t.Run(op.String(), func(t *testing.T) {
					testJMP(t, op, results[resOp][i], tc.items...)
				})
			}
		})
	}
}

func TestPUSHA(t *testing.T) {
	t.Run("Negative", getTestFuncForVM(makeProgram(opcode.PUSHA, 0xFF, 0xFF, 0xFF, 0xFF), nil))
	t.Run("TooBig", getTestFuncForVM(makeProgram(opcode.PUSHA, 10, 0, 0, 0), nil))
	t.Run("Good", func(t *testing.T) {
		prog := makeProgram(opcode.PUSHA, 2, 0, 0, 0)
		runWithArgs(t, prog, NewPointerItem(2, prog))
	})
}

func TestCALLA(t *testing.T) {
	prog := makeProgram(opcode.CALLA, opcode.PUSH2, opcode.ADD, opcode.RET, opcode.PUSH3, opcode.RET)
	t.Run("InvalidScript", getTestFuncForVM(prog, nil, NewPointerItem(4, []byte{1})))
	t.Run("Good", getTestFuncForVM(prog, 5, NewPointerItem(4, prog)))
}

func TestNOT(t *testing.T) {
	prog := makeProgram(opcode.NOT)
	t.Run("Bool", getTestFuncForVM(prog, true, false))
	t.Run("NonZeroInt", getTestFuncForVM(prog, false, 3))
	t.Run("Array", getTestFuncForVM(prog, false, []StackItem{}))
	t.Run("Struct", getTestFuncForVM(prog, false, NewStructItem([]StackItem{})))
	t.Run("ByteArray0", getTestFuncForVM(prog, true, []byte{0, 0}))
	t.Run("ByteArray1", getTestFuncForVM(prog, false, []byte{0, 1}))
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
}

// getBigInt returns 2^a+b
func getBigInt(a, b int64) *big.Int {
	p := new(big.Int).Exp(big.NewInt(2), big.NewInt(a), nil)
	p.Add(p, big.NewInt(b))
	return p
}

func TestArith(t *testing.T) {
	// results for 5 and 2
	testCases := map[opcode.Opcode]int{
		opcode.ADD: 7,
		opcode.MUL: 10,
		opcode.DIV: 2,
		opcode.MOD: 1,
		opcode.SUB: 3,
	}

	for op, res := range testCases {
		t.Run(op.String(), getTestFuncForVM(makeProgram(op), res, 5, 2))
	}
}

func TestADDBigResult(t *testing.T) {
	prog := makeProgram(opcode.ADD)
	runWithArgs(t, prog, nil, getBigInt(MaxBigIntegerSizeBits, -1), 1)
}

func TestMULBigResult(t *testing.T) {
	prog := makeProgram(opcode.MUL)
	bi := getBigInt(MaxBigIntegerSizeBits/2+1, 0)
	runWithArgs(t, prog, nil, bi, bi)
}

func TestArithNegativeArguments(t *testing.T) {
	runCase := func(op opcode.Opcode, p, q, result int64) func(t *testing.T) {
		return getTestFuncForVM(makeProgram(op), result, p, q)
	}

	t.Run("DIV", func(t *testing.T) {
		t.Run("positive/positive", runCase(opcode.DIV, 5, 2, 2))
		t.Run("positive/negative", runCase(opcode.DIV, 5, -2, -2))
		t.Run("negative/positive", runCase(opcode.DIV, -5, 2, -2))
		t.Run("negative/negative", runCase(opcode.DIV, -5, -2, 2))
	})

	t.Run("MOD", func(t *testing.T) {
		t.Run("positive/positive", runCase(opcode.MOD, 5, 2, 1))
		t.Run("positive/negative", runCase(opcode.MOD, 5, -2, 1))
		t.Run("negative/positive", runCase(opcode.MOD, -5, 2, -1))
		t.Run("negative/negative", runCase(opcode.MOD, -5, -2, -1))
	})

	t.Run("SHR", func(t *testing.T) {
		t.Run("positive/positive", runCase(opcode.SHR, 5, 2, 1))
		t.Run("negative/positive", runCase(opcode.SHR, -5, 2, -2))
	})

	t.Run("SHL", func(t *testing.T) {
		t.Run("positive/positive", runCase(opcode.SHL, 5, 2, 20))
		t.Run("negative/positive", runCase(opcode.SHL, -5, 2, -20))
	})
}

func TestSUBBigResult(t *testing.T) {
	prog := makeProgram(opcode.SUB)
	runWithArgs(t, prog, nil, getBigInt(MaxBigIntegerSizeBits, -1), -1)
}

func TestSHR(t *testing.T) {
	prog := makeProgram(opcode.SHR)
	t.Run("Good", getTestFuncForVM(prog, 1, 4, 2))
	t.Run("Zero", getTestFuncForVM(prog, []byte{0, 1}, []byte{0, 1}, 0))
	t.Run("Negative", getTestFuncForVM(prog, nil, 5, -1))
}

func TestSHL(t *testing.T) {
	prog := makeProgram(opcode.SHL)
	t.Run("Good", getTestFuncForVM(prog, 16, 4, 2))
	t.Run("Zero", getTestFuncForVM(prog, []byte{0, 1}, []byte{0, 1}, 0))
	t.Run("BigShift", getTestFuncForVM(prog, nil, 5, maxSHLArg+1))
	t.Run("BigResult", getTestFuncForVM(prog, nil, getBigInt(MaxBigIntegerSizeBits/2, 0), MaxBigIntegerSizeBits/2))
}

func TestLT(t *testing.T) {
	prog := makeProgram(opcode.LT)
	runWithArgs(t, prog, false, 4, 3)
}

func TestLTE(t *testing.T) {
	prog := makeProgram(opcode.LTE)
	runWithArgs(t, prog, true, 2, 3)
}

func TestGT(t *testing.T) {
	prog := makeProgram(opcode.GT)
	runWithArgs(t, prog, true, 9, 3)
}

func TestGTE(t *testing.T) {
	prog := makeProgram(opcode.GTE)
	runWithArgs(t, prog, true, 3, 3)
}

func TestDepth(t *testing.T) {
	prog := makeProgram(opcode.DEPTH)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, int64(3), vm.estack.Pop().BigInt().Int64())
}

func TestEQUALTrue(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.EQUAL)
	t.Run("Array", getTestFuncForVM(prog, true, []StackItem{}))
	t.Run("Map", getTestFuncForVM(prog, true, NewMapItem()))
}

func TestEQUAL(t *testing.T) {
	prog := makeProgram(opcode.EQUAL)
	t.Run("NoArgs", getTestFuncForVM(prog, nil))
	t.Run("OneArgument", getTestFuncForVM(prog, nil, 1))
	t.Run("Integer", getTestFuncForVM(prog, true, 5, 5))
	t.Run("IntegerByteArray", getTestFuncForVM(prog, true, []byte{16}, 16))
	t.Run("Map", getTestFuncForVM(prog, false, NewMapItem(), NewMapItem()))
	t.Run("Array", getTestFuncForVM(prog, false, []StackItem{}, []StackItem{}))
}

func runWithArgs(t *testing.T, prog []byte, result interface{}, args ...interface{}) {
	getTestFuncForVM(prog, result, args...)(t)
}

func getCustomTestFuncForVM(prog []byte, check func(t *testing.T, v *VM), args ...interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		v := load(prog)
		for i := range args {
			v.estack.PushVal(args[i])
		}
		if check == nil {
			checkVMFailed(t, v)
			return
		}
		runVM(t, v)
		check(t, v)
	}
}

func getTestFuncForVM(prog []byte, result interface{}, args ...interface{}) func(t *testing.T) {
	var f func(t *testing.T, v *VM)
	if result != nil {
		f = func(t *testing.T, v *VM) {
			require.Equal(t, 1, v.estack.Len())
			require.Equal(t, makeStackItem(result), v.estack.Pop().value)
		}
	}
	return getCustomTestFuncForVM(prog, f, args...)
}

func TestNOTEQUALByteArray(t *testing.T) {
	prog := makeProgram(opcode.NOTEQUAL)
	t.Run("True", getTestFuncForVM(prog, true, []byte{1, 2}, []byte{0, 1, 2}))
	t.Run("False", getTestFuncForVM(prog, false, []byte{1, 2}, []byte{1, 2}))
}

func TestNUMEQUAL(t *testing.T) {
	prog := makeProgram(opcode.NUMEQUAL)
	t.Run("True", getTestFuncForVM(prog, true, 2, 2))
	t.Run("False", getTestFuncForVM(prog, false, 1, 2))
}

func TestNUMNOTEQUAL(t *testing.T) {
	prog := makeProgram(opcode.NUMNOTEQUAL)
	t.Run("True", getTestFuncForVM(prog, true, 1, 2))
	t.Run("False", getTestFuncForVM(prog, false, 2, 2))
}

func TestINC(t *testing.T) {
	prog := makeProgram(opcode.INC)
	runWithArgs(t, prog, 2, 1)
}

func TestINCBigResult(t *testing.T) {
	prog := makeProgram(opcode.INC, opcode.INC)
	vm := load(prog)
	x := getBigInt(MaxBigIntegerSizeBits, -2)
	vm.estack.PushVal(x)

	require.NoError(t, vm.Step())
	require.False(t, vm.HasFailed())
	require.Equal(t, 1, vm.estack.Len())
	require.Equal(t, new(big.Int).Add(x, big.NewInt(1)), vm.estack.Top().BigInt())

	checkVMFailed(t, vm)
}

func TestDECBigResult(t *testing.T) {
	prog := makeProgram(opcode.DEC, opcode.DEC)
	vm := load(prog)
	x := getBigInt(MaxBigIntegerSizeBits, -2)
	x.Neg(x)
	vm.estack.PushVal(x)

	require.NoError(t, vm.Step())
	require.False(t, vm.HasFailed())
	require.Equal(t, 1, vm.estack.Len())
	require.Equal(t, new(big.Int).Sub(x, big.NewInt(1)), vm.estack.Top().BigInt())

	checkVMFailed(t, vm)
}

func TestNEWARRAY0(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY0)
	runWithArgs(t, prog, []StackItem{})
}

func TestNEWSTRUCT0(t *testing.T) {
	prog := makeProgram(opcode.NEWSTRUCT0)
	runWithArgs(t, prog, NewStructItem([]StackItem{}))
}

func TestNEWARRAYArray(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY)
	t.Run("ByteArray", getTestFuncForVM(prog, NewArrayItem([]StackItem{}), []byte{}))
	t.Run("BadSize", getTestFuncForVM(prog, nil, MaxArraySize+1))
	t.Run("Integer", getTestFuncForVM(prog, []StackItem{NewBoolItem(false)}, 1))

	arr := []StackItem{makeStackItem(42)}
	t.Run("Array", getTestFuncForVM(prog, arr, arr))
	t.Run("Struct", getTestFuncForVM(prog, arr, NewStructItem(arr)))
}

func testNEWARRAYIssue437(t *testing.T, i1, i2 opcode.Opcode, appended bool) {
	prog := makeProgram(
		opcode.PUSH2, i1,
		opcode.DUP, opcode.PUSH3, opcode.APPEND,
		opcode.TOALTSTACK, opcode.DUPFROMALTSTACK, i2,
		opcode.DUP, opcode.PUSH4, opcode.APPEND,
		opcode.FROMALTSTACK, opcode.PUSH5, opcode.APPEND)
	vm := load(prog)
	vm.Run()

	arr := makeArrayOfType(4, BooleanT)
	arr[2] = makeStackItem(3)
	arr[3] = makeStackItem(4)
	if appended {
		arr = append(arr, makeStackItem(5))
	}

	assert.Equal(t, false, vm.HasFailed())
	assert.Equal(t, 1, vm.estack.Len())
	if i2 == opcode.NEWARRAY {
		assert.Equal(t, &ArrayItem{arr}, vm.estack.Pop().value)
	} else {
		assert.Equal(t, &StructItem{arr}, vm.estack.Pop().value)
	}
}

func TestNEWARRAYIssue437(t *testing.T) {
	t.Run("Array+Array", func(t *testing.T) { testNEWARRAYIssue437(t, opcode.NEWARRAY, opcode.NEWARRAY, true) })
	t.Run("Struct+Struct", func(t *testing.T) { testNEWARRAYIssue437(t, opcode.NEWSTRUCT, opcode.NEWSTRUCT, true) })
	t.Run("Array+Struct", func(t *testing.T) { testNEWARRAYIssue437(t, opcode.NEWARRAY, opcode.NEWSTRUCT, false) })
	t.Run("Struct+Array", func(t *testing.T) { testNEWARRAYIssue437(t, opcode.NEWSTRUCT, opcode.NEWARRAY, false) })
}

func TestNEWARRAYT(t *testing.T) {
	testCases := map[StackItemType]StackItem{
		BooleanT:   NewBoolItem(false),
		IntegerT:   NewBigIntegerItem(big.NewInt(0)),
		ByteArrayT: NewByteArrayItem([]byte{}),
		ArrayT:     NullItem{},
		0xFF:       nil,
	}
	for typ, item := range testCases {
		prog := makeProgram(opcode.NEWARRAYT, opcode.Opcode(typ), opcode.PUSH0, opcode.PICKITEM)
		t.Run(typ.String(), getTestFuncForVM(prog, item, 1))
	}
}

func TestNEWSTRUCT(t *testing.T) {
	prog := makeProgram(opcode.NEWSTRUCT)
	t.Run("ByteArray", getTestFuncForVM(prog, NewStructItem([]StackItem{}), []byte{}))
	t.Run("BadSize", getTestFuncForVM(prog, nil, MaxArraySize+1))
	t.Run("Integer", getTestFuncForVM(prog, NewStructItem([]StackItem{NewBoolItem(false)}), 1))

	arr := []StackItem{makeStackItem(42)}
	t.Run("Array", getTestFuncForVM(prog, NewStructItem(arr), NewArrayItem(arr)))
	t.Run("Struct", getTestFuncForVM(prog, NewStructItem(arr), NewStructItem(arr)))
}

func TestAPPEND(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.PUSH5, opcode.APPEND)
	arr := []StackItem{makeStackItem(5)}
	t.Run("Array", getTestFuncForVM(prog, NewArrayItem(arr), NewArrayItem(nil)))
	t.Run("Struct", getTestFuncForVM(prog, NewStructItem(arr), NewStructItem(nil)))
}

func TestAPPENDCloneStruct(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.PUSH0, opcode.NEWSTRUCT, opcode.TOALTSTACK,
		opcode.DUPFROMALTSTACK, opcode.APPEND, opcode.FROMALTSTACK, opcode.PUSH1, opcode.APPEND)
	arr := []StackItem{&StructItem{[]StackItem{}}}
	runWithArgs(t, prog, NewArrayItem(arr), NewArrayItem(nil))
}

func TestAPPENDBad(t *testing.T) {
	prog := makeProgram(opcode.APPEND)
	t.Run("no arguments", getTestFuncForVM(prog, nil))
	t.Run("one argument", getTestFuncForVM(prog, nil, 1))
	t.Run("wrong type", getTestFuncForVM(prog, nil, []byte{}, 1))
}

func TestAPPENDGoodSizeLimit(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY, opcode.DUP, opcode.PUSH0, opcode.APPEND)
	vm := load(prog)
	vm.estack.PushVal(MaxArraySize - 1)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, MaxArraySize, len(vm.estack.Pop().Array()))
}

func TestAPPENDBadSizeLimit(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY, opcode.DUP, opcode.PUSH0, opcode.APPEND)
	runWithArgs(t, prog, nil, MaxArraySize)
}

func TestPICKITEM(t *testing.T) {
	prog := makeProgram(opcode.PICKITEM)
	t.Run("bad index", getTestFuncForVM(prog, nil, []StackItem{}, 0))
	t.Run("Array", getTestFuncForVM(prog, 2, []StackItem{makeStackItem(1), makeStackItem(2)}, 1))
	t.Run("ByteArray", getTestFuncForVM(prog, 2, []byte{1, 2}, 1))
}

func TestPICKITEMDupArray(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.PUSH0, opcode.PICKITEM, opcode.ABS)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{makeStackItem(-1)})
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, int64(1), vm.estack.Pop().BigInt().Int64())
	items := vm.estack.Pop().Value().([]StackItem)
	assert.Equal(t, big.NewInt(-1), items[0].Value())
}

func TestPICKITEMDupMap(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.PUSHINT8, 42, opcode.PICKITEM, opcode.ABS)
	vm := load(prog)
	m := NewMapItem()
	m.Add(makeStackItem([]byte{42}), makeStackItem(-1))
	vm.estack.Push(&Element{value: m})
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, int64(1), vm.estack.Pop().BigInt().Int64())
	items := vm.estack.Pop().Value().([]MapElement)
	assert.Equal(t, 1, len(items))
	assert.Equal(t, []byte{42}, items[0].Key.Value())
	assert.Equal(t, big.NewInt(-1), items[0].Value.Value())
}

func TestPICKITEMMap(t *testing.T) {
	prog := makeProgram(opcode.PICKITEM)
	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(3))
	runWithArgs(t, prog, 3, m, 5)
}

func TestSETITEMMap(t *testing.T) {
	prog := makeProgram(opcode.SETITEM, opcode.PICKITEM)
	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(3))
	runWithArgs(t, prog, []byte{0, 1}, m, 5, m, 5, []byte{0, 1})
}

func TestSETITEMBigMapBad(t *testing.T) {
	prog := makeProgram(opcode.SETITEM)
	m := NewMapItem()
	for i := 0; i < MaxArraySize; i++ {
		m.Add(makeStackItem(i), makeStackItem(i))
	}

	runWithArgs(t, prog, nil, m, MaxArraySize, 0)
}

func TestSETITEMBigMapGood(t *testing.T) {
	prog := makeProgram(opcode.SETITEM)
	vm := load(prog)

	m := NewMapItem()
	for i := 0; i < MaxArraySize; i++ {
		m.Add(makeStackItem(i), makeStackItem(i))
	}
	vm.estack.Push(&Element{value: m})
	vm.estack.PushVal(0)
	vm.estack.PushVal(0)

	runVM(t, vm)
}

func TestSIZE(t *testing.T) {
	prog := makeProgram(opcode.SIZE)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("ByteArray", getTestFuncForVM(prog, 2, []byte{0, 1}))
	t.Run("Bool", getTestFuncForVM(prog, 1, false))
	t.Run("Array", getTestFuncForVM(prog, 2, []StackItem{makeStackItem(1), makeStackItem([]byte{})}))
	t.Run("Map", func(t *testing.T) {
		m := NewMapItem()
		m.Add(makeStackItem(5), makeStackItem(6))
		m.Add(makeStackItem([]byte{0, 1}), makeStackItem(6))
		runWithArgs(t, prog, 2, m)
	})
}

func TestKEYSMap(t *testing.T) {
	prog := makeProgram(opcode.KEYS)
	vm := load(prog)

	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(6))
	m.Add(makeStackItem([]byte{0, 1}), makeStackItem(6))
	vm.estack.Push(&Element{value: m})

	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())

	top := vm.estack.Pop().value.(*ArrayItem)
	assert.Equal(t, 2, len(top.value))
	assert.Contains(t, top.value, makeStackItem(5))
	assert.Contains(t, top.value, makeStackItem([]byte{0, 1}))
}

func TestKEYS(t *testing.T) {
	prog := makeProgram(opcode.KEYS)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("WrongType", getTestFuncForVM(prog, nil, []StackItem{}))
}

func TestVALUESMap(t *testing.T) {
	prog := makeProgram(opcode.VALUES)
	vm := load(prog)

	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem([]byte{2, 3}))
	m.Add(makeStackItem([]byte{0, 1}), makeStackItem([]StackItem{}))
	vm.estack.Push(&Element{value: m})

	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())

	top := vm.estack.Pop().value.(*ArrayItem)
	assert.Equal(t, 2, len(top.value))
	assert.Contains(t, top.value, makeStackItem([]byte{2, 3}))
	assert.Contains(t, top.value, makeStackItem([]StackItem{}))
}

func TestVALUES(t *testing.T) {
	prog := makeProgram(opcode.VALUES)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("WrongType", getTestFuncForVM(prog, nil, 5))
	t.Run("Array", getTestFuncForVM(prog, []int{4}, []int{4}))
}

func TestHASKEY(t *testing.T) {
	prog := makeProgram(opcode.HASKEY)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("OneArgument", getTestFuncForVM(prog, nil, 1))
	t.Run("WrongKeyType", getTestFuncForVM(prog, nil, []StackItem{}, []StackItem{}))
	t.Run("WrongCollectionType", getTestFuncForVM(prog, nil, 1, 2))

	arr := makeArrayOfType(5, BooleanT)
	t.Run("Array", func(t *testing.T) {
		t.Run("True", getTestFuncForVM(prog, true, NewArrayItem(arr), 4))
		t.Run("False", getTestFuncForVM(prog, false, NewArrayItem(arr), 5))
	})
	t.Run("Struct", func(t *testing.T) {
		t.Run("True", getTestFuncForVM(prog, true, NewStructItem(arr), 4))
		t.Run("False", getTestFuncForVM(prog, false, NewStructItem(arr), 5))
	})
}

func TestHASKEYMap(t *testing.T) {
	prog := makeProgram(opcode.HASKEY)
	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(6))
	t.Run("True", getTestFuncForVM(prog, true, m, 5))
	t.Run("False", getTestFuncForVM(prog, false, m, 6))
}

func TestSIGN(t *testing.T) {
	prog := makeProgram(opcode.SIGN)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("WrongType", getTestFuncForVM(prog, nil, []StackItem{}))
	t.Run("Bool", getTestFuncForVM(prog, 0, false))
	t.Run("PositiveInt", getTestFuncForVM(prog, 1, 2))
	t.Run("NegativeInt", getTestFuncForVM(prog, -1, -1))
	t.Run("Zero", getTestFuncForVM(prog, 0, 0))
	t.Run("ByteArray", getTestFuncForVM(prog, 1, []byte{0, 1}))
}

func TestSimpleCall(t *testing.T) {
	buf := io.NewBufBinWriter()
	w := buf.BinWriter
	emit.Opcode(w, opcode.PUSH2)
	emit.Instruction(w, opcode.CALL, []byte{03})
	emit.Opcode(w, opcode.RET)
	emit.Opcode(w, opcode.PUSH10)
	emit.Opcode(w, opcode.ADD)
	emit.Opcode(w, opcode.RET)

	result := 12
	vm := load(buf.Bytes())
	runVM(t, vm)
	assert.Equal(t, result, int(vm.estack.Pop().BigInt().Int64()))
}

func TestNZ(t *testing.T) {
	prog := makeProgram(opcode.NZ)
	t.Run("True", getTestFuncForVM(prog, true, 1))
	t.Run("False", getTestFuncForVM(prog, false, 0))
}

func TestPICK(t *testing.T) {
	prog := makeProgram(opcode.PICK)
	t.Run("NoItem", getTestFuncForVM(prog, nil, 1))
	t.Run("Negative", getTestFuncForVM(prog, nil, -1))
}

func TestPICKgood(t *testing.T) {
	prog := makeProgram(opcode.PICK)
	result := 2
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal(1)
	vm.estack.PushVal(result)
	vm.estack.PushVal(3)
	vm.estack.PushVal(4)
	vm.estack.PushVal(5)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, int64(result), vm.estack.Pop().BigInt().Int64())
}

func TestPICKDup(t *testing.T) {
	prog := makeProgram(opcode.PUSHM1, opcode.PUSH0,
		opcode.PUSH1,
		opcode.PUSH2,
		opcode.PICK,
		opcode.ABS)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 4, vm.estack.Len())
	assert.Equal(t, int64(1), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(1), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(0), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(-1), vm.estack.Pop().BigInt().Int64())
}

func TestROTBad(t *testing.T) {
	prog := makeProgram(opcode.ROT)
	runWithArgs(t, prog, nil, 1, 2)
}

func TestROTGood(t *testing.T) {
	prog := makeProgram(opcode.ROT)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, 3, vm.estack.Len())
	assert.Equal(t, makeStackItem(1), vm.estack.Pop().value)
	assert.Equal(t, makeStackItem(3), vm.estack.Pop().value)
	assert.Equal(t, makeStackItem(2), vm.estack.Pop().value)
}

func TestROLLBad1(t *testing.T) {
	prog := makeProgram(opcode.ROLL)
	runWithArgs(t, prog, nil, 1, -1)
}

func TestROLLBad2(t *testing.T) {
	prog := makeProgram(opcode.ROLL)
	runWithArgs(t, prog, nil, 1, 2, 3, 3)
}

func TestROLLGood(t *testing.T) {
	prog := makeProgram(opcode.ROLL)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.estack.PushVal(4)
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 4, vm.estack.Len())
	assert.Equal(t, makeStackItem(3), vm.estack.Pop().value)
	assert.Equal(t, makeStackItem(4), vm.estack.Pop().value)
	assert.Equal(t, makeStackItem(2), vm.estack.Pop().value)
	assert.Equal(t, makeStackItem(1), vm.estack.Pop().value)
}

func getCheckEStackFunc(items ...interface{}) func(t *testing.T, v *VM) {
	return func(t *testing.T, v *VM) {
		require.Equal(t, len(items), v.estack.Len())
		for i := 0; i < len(items); i++ {
			assert.Equal(t, makeStackItem(items[i]), v.estack.Peek(i).Item())
		}
	}
}

func TestREVERSE3(t *testing.T) {
	prog := makeProgram(opcode.REVERSE3)
	t.Run("SmallStack", getTestFuncForVM(prog, nil, 1, 2))
	t.Run("Good", getCustomTestFuncForVM(prog, getCheckEStackFunc(1, 2, 3), 1, 2, 3))
}

func TestREVERSE4(t *testing.T) {
	prog := makeProgram(opcode.REVERSE4)
	t.Run("SmallStack", getTestFuncForVM(prog, nil, 1, 2, 3))
	t.Run("Good", getCustomTestFuncForVM(prog, getCheckEStackFunc(1, 2, 3, 4), 1, 2, 3, 4))
}

func TestREVERSEN(t *testing.T) {
	prog := makeProgram(opcode.REVERSEN)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("SmallStack", getTestFuncForVM(prog, nil, 1, 2, 3))
	t.Run("NegativeArgument", getTestFuncForVM(prog, nil, 1, 2, -1))
	t.Run("Zero", getCustomTestFuncForVM(prog, getCheckEStackFunc(3, 2, 1), 1, 2, 3, 0))
	t.Run("OneItem", getCustomTestFuncForVM(prog, getCheckEStackFunc(42), 42, 1))
	t.Run("Good", getCustomTestFuncForVM(prog, getCheckEStackFunc(1, 2, 3, 4, 5), 1, 2, 3, 4, 5, 5))
}

func TestTUCKbadNoitems(t *testing.T) {
	prog := makeProgram(opcode.TUCK)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("NoItem", getTestFuncForVM(prog, nil, 1))
}

func TestTUCKgood(t *testing.T) {
	prog := makeProgram(opcode.TUCK)
	vm := load(prog)
	vm.estack.PushVal(42)
	vm.estack.PushVal(34)
	runVM(t, vm)
	assert.Equal(t, int64(34), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(42), vm.estack.Peek(1).BigInt().Int64())
	assert.Equal(t, int64(34), vm.estack.Peek(2).BigInt().Int64())
}

func TestTUCKgood2(t *testing.T) {
	prog := makeProgram(opcode.TUCK)
	vm := load(prog)
	vm.estack.PushVal(11)
	vm.estack.PushVal(42)
	vm.estack.PushVal(34)
	runVM(t, vm)
	assert.Equal(t, int64(34), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(42), vm.estack.Peek(1).BigInt().Int64())
	assert.Equal(t, int64(34), vm.estack.Peek(2).BigInt().Int64())
	assert.Equal(t, int64(11), vm.estack.Peek(3).BigInt().Int64())
}

func TestOVER(t *testing.T) {
	prog := makeProgram(opcode.OVER)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("NoItem", getTestFuncForVM(prog, nil, 1))
}

func TestOVERgood(t *testing.T) {
	prog := makeProgram(opcode.OVER)
	vm := load(prog)
	vm.estack.PushVal(42)
	vm.estack.PushVal(34)
	runVM(t, vm)
	assert.Equal(t, int64(42), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(34), vm.estack.Peek(1).BigInt().Int64())
	assert.Equal(t, int64(42), vm.estack.Peek(2).BigInt().Int64())
	assert.Equal(t, 3, vm.estack.Len())
}

func TestOVERDup(t *testing.T) {
	prog := makeProgram(opcode.PUSHDATA1, 2, 1, 0,
		opcode.PUSH1,
		opcode.OVER,
		opcode.PUSH1,
		opcode.LEFT,
		opcode.PUSHDATA1, 1, 2,
		opcode.CAT)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 3, vm.estack.Len())
	assert.Equal(t, []byte{0x01, 0x02}, vm.estack.Pop().Bytes())
	assert.Equal(t, int64(1), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, []byte{0x01, 0x00}, vm.estack.Pop().Bytes())
}

func TestNIPBadNoItem(t *testing.T) {
	prog := makeProgram(opcode.NIP)
	runWithArgs(t, prog, nil, 1)
}

func TestNIPGood(t *testing.T) {
	prog := makeProgram(opcode.NIP)
	runWithArgs(t, prog, 2, 1, 2)
}

func TestDROPBadNoItem(t *testing.T) {
	prog := makeProgram(opcode.DROP)
	runWithArgs(t, prog, nil)
}

func TestDROPGood(t *testing.T) {
	prog := makeProgram(opcode.DROP)
	vm := load(prog)
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 0, vm.estack.Len())
}

func TestXDROP(t *testing.T) {
	prog := makeProgram(opcode.XDROP)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("NoN", getTestFuncForVM(prog, nil, 1, 2))
	t.Run("Negative", getTestFuncForVM(prog, nil, 1, -1))
}

func TestXDROPgood(t *testing.T) {
	prog := makeProgram(opcode.XDROP)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, int64(2), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(1), vm.estack.Peek(1).BigInt().Int64())
}

func TestCLEAR(t *testing.T) {
	prog := makeProgram(opcode.CLEAR)
	v := load(prog)
	v.estack.PushVal(123)
	require.Equal(t, 1, v.estack.Len())
	require.NoError(t, v.Run())
	require.Equal(t, 0, v.estack.Len())
}

func TestINVERTbadNoitem(t *testing.T) {
	prog := makeProgram(opcode.INVERT)
	runWithArgs(t, prog, nil)
}

func TestINVERTgood1(t *testing.T) {
	prog := makeProgram(opcode.INVERT)
	runWithArgs(t, prog, -1, 0)
}

func TestINVERTgood2(t *testing.T) {
	prog := makeProgram(opcode.INVERT)
	vm := load(prog)
	vm.estack.PushVal(-1)
	runVM(t, vm)
	assert.Equal(t, int64(0), vm.estack.Peek(0).BigInt().Int64())
}

func TestINVERTgood3(t *testing.T) {
	prog := makeProgram(opcode.INVERT)
	vm := load(prog)
	vm.estack.PushVal(0x69)
	runVM(t, vm)
	assert.Equal(t, int64(-0x6A), vm.estack.Peek(0).BigInt().Int64())
}

func TestINVERTWithConversion1(t *testing.T) {
	prog := makeProgram(opcode.PUSHDATA2, 0, 0, opcode.INVERT)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, int64(-1), vm.estack.Peek(0).BigInt().Int64())
}

func TestINVERTWithConversion2(t *testing.T) {
	prog := makeProgram(opcode.PUSH0, opcode.PUSH1, opcode.NUMEQUAL, opcode.INVERT)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, int64(-1), vm.estack.Peek(0).BigInt().Int64())
}

func TestCAT(t *testing.T) {
	prog := makeProgram(opcode.CAT)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("OneArgument", getTestFuncForVM(prog, nil, []byte("abc")))
	t.Run("BigItem", func(t *testing.T) {
		arg := make([]byte, MaxItemSize/2+1)
		runWithArgs(t, prog, nil, arg, arg)
	})
	t.Run("Good", getTestFuncForVM(prog, []byte("abcdef"), []byte("abc"), []byte("def")))
	t.Run("Int0ByteArray", getTestFuncForVM(prog, []byte{}, 0, []byte{}))
	t.Run("ByteArrayInt1", getTestFuncForVM(prog, []byte{1}, []byte{}, 1))
}

func TestSUBSTR(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("OneArgument", getTestFuncForVM(prog, nil, 1))
	t.Run("TwoArguments", getTestFuncForVM(prog, nil, 0, 2))
	t.Run("Good", getTestFuncForVM(prog, []byte("bc"), []byte("abcdef"), 1, 2))
	t.Run("BadOffset", getTestFuncForVM(prog, nil, []byte("abcdef"), 7, 1))
	t.Run("BigLen", getTestFuncForVM(prog, nil, []byte("abcdef"), 1, 6))
	t.Run("NegativeOffset", getTestFuncForVM(prog, nil, []byte("abcdef"), -1, 3))
	t.Run("NegativeLen", getTestFuncForVM(prog, nil, []byte("abcdef"), 3, -1))
}

func TestSUBSTRBad387(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	b := make([]byte, 6, 20)
	copy(b, "abcdef")
	vm.estack.PushVal(b)
	vm.estack.PushVal(1)
	vm.estack.PushVal(6)
	checkVMFailed(t, vm)
}

func TestLEFT(t *testing.T) {
	prog := makeProgram(opcode.LEFT)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("NoString", getTestFuncForVM(prog, nil, 2))
	t.Run("NegativeLen", getTestFuncForVM(prog, nil, "abcdef", -1))
	t.Run("Good", getTestFuncForVM(prog, "ab", "abcdef", 2))
	t.Run("GoodBigLen", getTestFuncForVM(prog, "abcdef", "abcdef", 8))
}

func TestRIGHT(t *testing.T) {
	prog := makeProgram(opcode.RIGHT)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("NoString", getTestFuncForVM(prog, nil, 2))
	t.Run("NegativeLen", getTestFuncForVM(prog, nil, "abcdef", -1))
	t.Run("Good", getTestFuncForVM(prog, "ef", "abcdef", 2))
	t.Run("BadLen", getTestFuncForVM(prog, nil, "abcdef", 8))
}

func TestPACK(t *testing.T) {
	prog := makeProgram(opcode.PACK)
	t.Run("BadLen", getTestFuncForVM(prog, nil, 1))
	t.Run("Good0Len", getTestFuncForVM(prog, []StackItem{}, 0))
}

func TestPACKBigLen(t *testing.T) {
	prog := makeProgram(opcode.PACK)
	vm := load(prog)
	for i := 0; i <= MaxArraySize; i++ {
		vm.estack.PushVal(0)
	}
	vm.estack.PushVal(MaxArraySize + 1)
	checkVMFailed(t, vm)
}

func TestPACKGood(t *testing.T) {
	prog := makeProgram(opcode.PACK)
	elements := []int{55, 34, 42}
	vm := load(prog)
	// canary
	vm.estack.PushVal(1)
	for i := len(elements) - 1; i >= 0; i-- {
		vm.estack.PushVal(elements[i])
	}
	vm.estack.PushVal(len(elements))
	runVM(t, vm)
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
	prog := makeProgram(opcode.UNPACK)
	runWithArgs(t, prog, nil, 1)
}

func TestUNPACKGood(t *testing.T) {
	prog := makeProgram(opcode.UNPACK)
	elements := []int{55, 34, 42}
	vm := load(prog)
	// canary
	vm.estack.PushVal(1)
	vm.estack.PushVal(elements)
	runVM(t, vm)
	assert.Equal(t, 5, vm.estack.Len())
	assert.Equal(t, int64(len(elements)), vm.estack.Peek(0).BigInt().Int64())
	for k, v := range elements {
		assert.Equal(t, int64(v), vm.estack.Peek(k+1).BigInt().Int64())
	}
	assert.Equal(t, int64(1), vm.estack.Peek(len(elements)+1).BigInt().Int64())
}

func TestREVERSEITEMSBadNotArray(t *testing.T) {
	prog := makeProgram(opcode.REVERSEITEMS)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func testREVERSEITEMSIssue437(t *testing.T, i1, i2 opcode.Opcode, reversed bool) {
	prog := makeProgram(
		opcode.PUSH0, i1,
		opcode.DUP, opcode.PUSH1, opcode.APPEND,
		opcode.DUP, opcode.PUSH2, opcode.APPEND,
		opcode.DUP, i2, opcode.REVERSEITEMS)
	vm := load(prog)
	vm.Run()

	arr := make([]StackItem, 2)
	if reversed {
		arr[0] = makeStackItem(2)
		arr[1] = makeStackItem(1)
	} else {
		arr[0] = makeStackItem(1)
		arr[1] = makeStackItem(2)
	}
	assert.Equal(t, false, vm.HasFailed())
	assert.Equal(t, 1, vm.estack.Len())
	if i1 == opcode.NEWARRAY {
		assert.Equal(t, &ArrayItem{arr}, vm.estack.Pop().value)
	} else {
		assert.Equal(t, &StructItem{arr}, vm.estack.Pop().value)
	}
}

func TestREVERSEITEMSIssue437(t *testing.T) {
	t.Run("Array+Array", func(t *testing.T) { testREVERSEITEMSIssue437(t, opcode.NEWARRAY, opcode.NEWARRAY, true) })
	t.Run("Struct+Struct", func(t *testing.T) { testREVERSEITEMSIssue437(t, opcode.NEWSTRUCT, opcode.NEWSTRUCT, true) })
	t.Run("Array+Struct", func(t *testing.T) { testREVERSEITEMSIssue437(t, opcode.NEWARRAY, opcode.NEWSTRUCT, false) })
	t.Run("Struct+Array", func(t *testing.T) { testREVERSEITEMSIssue437(t, opcode.NEWSTRUCT, opcode.NEWARRAY, false) })
}

func TestREVERSEITEMSGoodOneElem(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.REVERSEITEMS)
	elements := []int{22}
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(elements)
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	a := vm.estack.Peek(0).Array()
	assert.Equal(t, len(elements), len(a))
	e := a[0].Value().(*big.Int)
	assert.Equal(t, int64(elements[0]), e.Int64())
}

func TestREVERSEITEMSGoodStruct(t *testing.T) {
	eodd := []int{22, 34, 42, 55, 81}
	even := []int{22, 34, 42, 55, 81, 99}
	eall := [][]int{eodd, even}

	for _, elements := range eall {
		prog := makeProgram(opcode.DUP, opcode.REVERSEITEMS)
		vm := load(prog)
		vm.estack.PushVal(1)

		arr := make([]StackItem, len(elements))
		for i := range elements {
			arr[i] = makeStackItem(elements[i])
		}
		vm.estack.Push(&Element{value: &StructItem{arr}})

		runVM(t, vm)
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

func TestREVERSEITEMSGood(t *testing.T) {
	eodd := []int{22, 34, 42, 55, 81}
	even := []int{22, 34, 42, 55, 81, 99}
	eall := [][]int{eodd, even}

	for _, elements := range eall {
		prog := makeProgram(opcode.DUP, opcode.REVERSEITEMS)
		vm := load(prog)
		vm.estack.PushVal(1)
		vm.estack.PushVal(elements)
		runVM(t, vm)
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

func TestREMOVE(t *testing.T) {
	prog := makeProgram(opcode.REMOVE)
	t.Run("NoArgument", getTestFuncForVM(prog, nil))
	t.Run("OneArgument", getTestFuncForVM(prog, nil, 1))
	t.Run("NotArray", getTestFuncForVM(prog, nil, 1, 1))
	t.Run("BadIndex", getTestFuncForVM(prog, nil, []int{22, 34, 42, 55, 81}, 10))
}

func TestREMOVEGood(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.PUSH2, opcode.REMOVE)
	elements := []int{22, 34, 42, 55, 81}
	reselements := []int{22, 34, 55, 81}
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(elements)
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, makeStackItem(reselements), vm.estack.Pop().value)
	assert.Equal(t, makeStackItem(1), vm.estack.Pop().value)
}

func TestREMOVEMap(t *testing.T) {
	prog := makeProgram(opcode.REMOVE, opcode.PUSH5, opcode.HASKEY)
	vm := load(prog)

	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(3))
	m.Add(makeStackItem([]byte{0, 1}), makeStackItem([]byte{2, 3}))
	vm.estack.Push(&Element{value: m})
	vm.estack.Push(&Element{value: m})
	vm.estack.PushVal(makeStackItem(5))

	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(false), vm.estack.Pop().value)
}

func testCLEARITEMS(t *testing.T, item StackItem) {
	prog := makeProgram(opcode.DUP, opcode.DUP, opcode.CLEARITEMS, opcode.SIZE)
	v := load(prog)
	v.estack.PushVal(item)
	runVM(t, v)
	require.Equal(t, 2, v.estack.Len())
	require.EqualValues(t, 2, v.refs.size) // empty collection + it's size
	require.EqualValues(t, 0, v.estack.Pop().BigInt().Int64())
}

func TestCLEARITEMS(t *testing.T) {
	arr := []StackItem{NewBigIntegerItem(big.NewInt(1)), NewByteArrayItem([]byte{1})}
	m := NewMapItem()
	m.Add(NewBigIntegerItem(big.NewInt(1)), NewByteArrayItem([]byte{}))
	m.Add(NewByteArrayItem([]byte{42}), NewBigIntegerItem(big.NewInt(2)))

	testCases := map[string]StackItem{
		"empty Array":   NewArrayItem([]StackItem{}),
		"filled Array":  NewArrayItem(arr),
		"empty Struct":  NewStructItem([]StackItem{}),
		"filled Struct": NewStructItem(arr),
		"empty Map":     NewMapItem(),
		"filled Map":    m,
	}

	for name, item := range testCases {
		t.Run(name, func(t *testing.T) { testCLEARITEMS(t, item) })
	}

	t.Run("Integer", func(t *testing.T) {
		prog := makeProgram(opcode.CLEARITEMS)
		runWithArgs(t, prog, nil, 1)
	})
}

func TestSWAPGood(t *testing.T) {
	prog := makeProgram(opcode.SWAP)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.estack.PushVal(4)
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, int64(2), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(4), vm.estack.Pop().BigInt().Int64())
}

func TestSWAP(t *testing.T) {
	prog := makeProgram(opcode.SWAP)
	t.Run("EmptyStack", getTestFuncForVM(prog, nil))
	t.Run("SmallStack", getTestFuncForVM(prog, nil, 4))
}

func TestDupInt(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.ABS)
	vm := load(prog)
	vm.estack.PushVal(-1)
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, int64(1), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(-1), vm.estack.Pop().BigInt().Int64())
}

func TestDupByteArray(t *testing.T) {
	prog := makeProgram(opcode.PUSHDATA1, 2, 1, 0,
		opcode.DUP,
		opcode.PUSH1,
		opcode.LEFT,
		opcode.PUSHDATA1, 1, 2,
		opcode.CAT)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, []byte{0x01, 0x02}, vm.estack.Pop().Bytes())
	assert.Equal(t, []byte{0x01, 0x00}, vm.estack.Pop().Bytes())
}

func TestDupBool(t *testing.T) {
	prog := makeProgram(opcode.PUSH0, opcode.NOT,
		opcode.DUP,
		opcode.PUSH1, opcode.NOT,
		opcode.BOOLAND)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 2, vm.estack.Len())
	assert.Equal(t, false, vm.estack.Pop().Bool())
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

var opcodesTestCases = map[opcode.Opcode][]struct {
	name     string
	args     []interface{}
	expected interface{}
	actual   func(vm *VM) interface{}
}{
	opcode.AND: {
		{
			name:     "1_1",
			args:     []interface{}{1, 1},
			expected: int64(1),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "1_0",
			args:     []interface{}{1, 0},
			expected: int64(0),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "0_1",
			args:     []interface{}{0, 1},
			expected: int64(0),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "0_0",
			args:     []interface{}{0, 0},
			expected: int64(0),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name: "random_values",
			args: []interface{}{
				[]byte{1, 0, 1, 0, 1, 0, 1, 1},
				[]byte{1, 1, 0, 0, 0, 0, 0, 1},
			},
			expected: []byte{1, 0, 0, 0, 0, 0, 0, 1},
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bytes()
			},
		},
	},
	opcode.OR: {
		{
			name:     "1_1",
			args:     []interface{}{1, 1},
			expected: int64(1),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "0_0",
			args:     []interface{}{0, 0},
			expected: int64(0),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "0_1",
			args:     []interface{}{0, 1},
			expected: int64(1),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "1_0",
			args:     []interface{}{1, 0},
			expected: int64(1),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name: "random_values",
			args: []interface{}{
				[]byte{1, 0, 1, 0, 1, 0, 1, 1},
				[]byte{1, 1, 0, 0, 0, 0, 0, 1},
			},
			expected: []byte{1, 1, 1, 0, 1, 0, 1, 1},
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bytes()
			},
		},
	},
	opcode.XOR: {
		{
			name:     "1_1",
			args:     []interface{}{1, 1},
			expected: int64(0),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "0_0",
			args:     []interface{}{0, 0},
			expected: int64(0),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "0_1",
			args:     []interface{}{0, 1},
			expected: int64(1),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "1_0",
			args:     []interface{}{1, 0},
			expected: int64(1),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name: "random_values",
			args: []interface{}{
				[]byte{1, 0, 1, 0, 1, 0, 1, 1},
				[]byte{1, 1, 0, 0, 0, 0, 0, 1},
			},
			expected: []byte{0, 1, 1, 0, 1, 0, 1},
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bytes()
			},
		},
	},
	opcode.BOOLOR: {
		{
			name:     "1_1",
			args:     []interface{}{true, true},
			expected: true,
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bool()
			},
		},
		{
			name:     "0_0",
			args:     []interface{}{false, false},
			expected: false,
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bool()
			},
		},
		{
			name:     "0_1",
			args:     []interface{}{false, true},
			expected: true,
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bool()
			},
		},
		{
			name:     "1_0",
			args:     []interface{}{true, false},
			expected: true,
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bool()
			},
		},
	},
	opcode.MIN: {
		{
			name:     "3_5",
			args:     []interface{}{3, 5},
			expected: int64(3),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "5_3",
			args:     []interface{}{5, 3},
			expected: int64(3),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "3_3",
			args:     []interface{}{3, 3},
			expected: int64(3),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
	},
	opcode.MAX: {
		{
			name:     "3_5",
			args:     []interface{}{3, 5},
			expected: int64(5),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "5_3",
			args:     []interface{}{5, 3},
			expected: int64(5),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "3_3",
			args:     []interface{}{3, 3},
			expected: int64(3),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
	},
	opcode.WITHIN: {
		{
			name:     "within",
			args:     []interface{}{4, 3, 5},
			expected: true,
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bool()
			},
		},
		{
			name:     "less",
			args:     []interface{}{2, 3, 5},
			expected: false,
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bool()
			},
		},
		{
			name:     "more",
			args:     []interface{}{6, 3, 5},
			expected: false,
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().Bool()
			},
		},
	},
	opcode.NEGATE: {
		{
			name:     "3",
			args:     []interface{}{3},
			expected: int64(-3),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "-3",
			args:     []interface{}{-3},
			expected: int64(3),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
		{
			name:     "0",
			args:     []interface{}{0},
			expected: int64(0),
			actual: func(vm *VM) interface{} {
				return vm.estack.Pop().BigInt().Int64()
			},
		},
	},
}

func TestBitAndNumericOpcodes(t *testing.T) {
	for code, opcodeTestCases := range opcodesTestCases {
		t.Run(code.String(), func(t *testing.T) {
			for _, testCase := range opcodeTestCases {
				prog := makeProgram(code)
				vm := load(prog)
				t.Run(testCase.name, func(t *testing.T) {
					for _, arg := range testCase.args {
						vm.estack.PushVal(arg)
					}
					runVM(t, vm)
					assert.Equal(t, testCase.expected, testCase.actual(vm))
				})
			}
		})
	}
}

func makeProgram(opcodes ...opcode.Opcode) []byte {
	prog := make([]byte, len(opcodes)+1) // RET
	for i := 0; i < len(opcodes); i++ {
		prog[i] = byte(opcodes[i])
	}
	prog[len(prog)-1] = byte(opcode.RET)
	return prog
}

func load(prog []byte) *VM {
	vm := New()
	vm.LoadScript(prog)
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
