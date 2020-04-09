package vm

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fooInteropGetter(id uint32) *InteropFuncPrice {
	if id == InteropNameToID([]byte("foo")) {
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

func TestInteropHookViaID(t *testing.T) {
	v := New()
	v.RegisterInteropGetter(fooInteropGetter)

	buf := io.NewBufBinWriter()
	fooid := InteropNameToID([]byte("foo"))
	var id = make([]byte, 4)
	binary.LittleEndian.PutUint32(id, fooid)
	emit.Syscall(buf.BinWriter, string(id))
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

func TestPushBytesNoParam(t *testing.T) {
	prog := make([]byte, 1)
	prog[0] = byte(opcode.PUSHBYTES1)
	vm := load(prog)
	checkVMFailed(t, vm)
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
		opcode.PUSHBYTES2, opcode.Opcode(size), opcode.Opcode(size>>8), // LE
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
		require.Equal(t, expected[i].size, vm.size)
	}
}

func TestPushBytesShort(t *testing.T) {
	prog := make([]byte, 10)
	prog[0] = byte(opcode.PUSHBYTES10) // but only 9 left in the `prog`
	vm := load(prog)
	checkVMFailed(t, vm)
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
		if i == 80 {
			continue // nice opcode layout we got here.
		}
		err := vm.Step()
		require.NoError(t, err)

		elem := vm.estack.Pop()
		assert.IsType(t, &BigIntegerItem{}, elem.value)
		val := i - int(opcode.PUSH1) + 1
		assert.Equal(t, elem.BigInt().Int64(), int64(val))
	}
}

func TestPushData1BadNoN(t *testing.T) {
	prog := []byte{byte(opcode.PUSHDATA1)}
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestPushData1BadN(t *testing.T) {
	prog := []byte{byte(opcode.PUSHDATA1), 1}
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestPushData1Good(t *testing.T) {
	prog := makeProgram(opcode.PUSHDATA1, 3, 1, 2, 3)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte{1, 2, 3}, vm.estack.Pop().Bytes())
}

func TestPushData2BadNoN(t *testing.T) {
	prog := []byte{byte(opcode.PUSHDATA2)}
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestPushData2ShortN(t *testing.T) {
	prog := []byte{byte(opcode.PUSHDATA2), 0}
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestPushData2BadN(t *testing.T) {
	prog := []byte{byte(opcode.PUSHDATA2), 1, 0}
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestPushData2Good(t *testing.T) {
	prog := makeProgram(opcode.PUSHDATA2, 3, 0, 1, 2, 3)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte{1, 2, 3}, vm.estack.Pop().Bytes())
}

func TestPushData4BadNoN(t *testing.T) {
	prog := []byte{byte(opcode.PUSHDATA4)}
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestPushData4BadN(t *testing.T) {
	prog := []byte{byte(opcode.PUSHDATA4), 1, 0, 0, 0}
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestPushData4ShortN(t *testing.T) {
	prog := []byte{byte(opcode.PUSHDATA4), 0, 0, 0}
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestPushData4BigN(t *testing.T) {
	prog := make([]byte, 1+4+MaxItemSize+1)
	prog[0] = byte(opcode.PUSHDATA4)
	binary.LittleEndian.PutUint32(prog[1:], MaxItemSize+1)

	vm := load(prog)
	vm.Run()
	assert.Equal(t, true, vm.HasFailed())
}

func TestPushData4Good(t *testing.T) {
	prog := makeProgram(opcode.PUSHDATA4, 3, 0, 0, 0, 1, 2, 3)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte{1, 2, 3}, vm.estack.Pop().Bytes())
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
		NewBigIntegerItem(42),
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
		NewBigIntegerItem(123),
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
		NewBigIntegerItem(42),
	})
	v.estack.PushVal(arr)

	runVM(t, v)

	checkEnumeratorStack(t, v, []StackItem{
		NewBigIntegerItem(1), NewBoolItem(true),
		NewBigIntegerItem(0), NewBoolItem(true),
	})
}

func TestIteratorValues(t *testing.T) {
	prog := getSyscallProg("Neo.Iterator.Create")
	prog = append(prog, getSyscallProg("Neo.Iterator.Values")...)
	prog = append(prog, byte(opcode.TOALTSTACK), byte(opcode.DUPFROMALTSTACK))
	prog = append(prog, getEnumeratorProg(2, false)...)

	v := load(prog)
	m := NewMapItem()
	m.Add(NewBigIntegerItem(1), NewBoolItem(false))
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
	prog = []byte{byte(opcode.SYSCALL)}
	prog = append(prog, byte(len(name)))
	prog = append(prog, name...)

	return
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
	item := NewArrayItem(makeArrayOfFalses(2))
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
	vm := load(prog)

	data, err := SerializeItem(NewBigIntegerItem(123))
	require.NoError(t, err)

	data[0] = 0xFF
	vm.estack.PushVal(data)

	checkVMFailed(t, vm)
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
	// Create a map, push key and value, add KV to map, serialize.
	progHex := "c776036b65790576616c7565c468154e656f2e52756e74696d652e53657269616c697a65"
	resHex := "820100036b6579000576616c7565"
	prog, err := hex.DecodeString(progHex)
	require.NoError(t, err)
	res, err := hex.DecodeString(resHex)
	require.NoError(t, err)

	vm := load(prog)
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
		opcode.PUSHBYTES2, opcode.Opcode(n), opcode.Opcode(n>>8), // little-endian
		opcode.TOALTSTACK, opcode.DUPFROMALTSTACK,
		opcode.JMPIF, 0x4, 0, opcode.RET,
		opcode.FROMALTSTACK, opcode.DEC,
		opcode.CALL, 0xF8, 0xFF) // -8 -> JMP to TOALTSTACK)
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

func TestNOTNoArgument(t *testing.T) {
	prog := makeProgram(opcode.NOT)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestNOTBool(t *testing.T) {
	prog := makeProgram(opcode.NOT)
	vm := load(prog)
	vm.estack.PushVal(false)
	runVM(t, vm)
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestNOTNonZeroInt(t *testing.T) {
	prog := makeProgram(opcode.NOT)
	vm := load(prog)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestNOTArray(t *testing.T) {
	prog := makeProgram(opcode.NOT)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	runVM(t, vm)
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestNOTStruct(t *testing.T) {
	prog := makeProgram(opcode.NOT)
	vm := load(prog)
	vm.estack.Push(NewElement(&StructItem{[]StackItem{}}))
	runVM(t, vm)
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestNOTByteArray0(t *testing.T) {
	prog := makeProgram(opcode.NOT)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 0})
	runVM(t, vm)
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestNOTByteArray1(t *testing.T) {
	prog := makeProgram(opcode.NOT)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 1})
	runVM(t, vm)
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

// getBigInt returns 2^a+b
func getBigInt(a, b int64) *big.Int {
	p := new(big.Int).Exp(big.NewInt(2), big.NewInt(a), nil)
	p.Add(p, big.NewInt(b))
	return p
}

func TestAdd(t *testing.T) {
	prog := makeProgram(opcode.ADD)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, int64(6), vm.estack.Pop().BigInt().Int64())
}

func TestADDBigResult(t *testing.T) {
	prog := makeProgram(opcode.ADD)
	vm := load(prog)
	vm.estack.PushVal(getBigInt(MaxBigIntegerSizeBits, -1))
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func testBigArgument(t *testing.T, inst opcode.Opcode) {
	prog := makeProgram(inst)
	x := getBigInt(MaxBigIntegerSizeBits, 0)
	t.Run(inst.String()+" big 1-st argument", func(t *testing.T) {
		vm := load(prog)
		vm.estack.PushVal(x)
		vm.estack.PushVal(0)
		checkVMFailed(t, vm)
	})
	t.Run(inst.String()+" big 2-nd argument", func(t *testing.T) {
		vm := load(prog)
		vm.estack.PushVal(0)
		vm.estack.PushVal(x)
		checkVMFailed(t, vm)
	})
}

func TestArithBigArgument(t *testing.T) {
	testBigArgument(t, opcode.ADD)
	testBigArgument(t, opcode.SUB)
	testBigArgument(t, opcode.MUL)
	testBigArgument(t, opcode.DIV)
	testBigArgument(t, opcode.MOD)
}

func TestMul(t *testing.T) {
	prog := makeProgram(opcode.MUL)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, int64(8), vm.estack.Pop().BigInt().Int64())
}

func TestMULBigResult(t *testing.T) {
	prog := makeProgram(opcode.MUL)
	vm := load(prog)
	vm.estack.PushVal(getBigInt(MaxBigIntegerSizeBits/2+1, 0))
	vm.estack.PushVal(getBigInt(MaxBigIntegerSizeBits/2+1, 0))
	checkVMFailed(t, vm)
}

func TestArithNegativeArguments(t *testing.T) {
	runCase := func(op opcode.Opcode, p, q, result int64) func(t *testing.T) {
		return func(t *testing.T) {
			vm := load(makeProgram(op))
			vm.estack.PushVal(p)
			vm.estack.PushVal(q)
			runVM(t, vm)
			assert.Equal(t, result, vm.estack.Pop().BigInt().Int64())
		}
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
		t.Run("positive/negative", runCase(opcode.SHR, 5, -2, 20))
		t.Run("negative/positive", runCase(opcode.SHR, -5, 2, -2))
		t.Run("negative/negative", runCase(opcode.SHR, -5, -2, -20))
	})

	t.Run("SHL", func(t *testing.T) {
		t.Run("positive/positive", runCase(opcode.SHL, 5, 2, 20))
		t.Run("positive/negative", runCase(opcode.SHL, 5, -2, 1))
		t.Run("negative/positive", runCase(opcode.SHL, -5, 2, -20))
		t.Run("negative/negative", runCase(opcode.SHL, -5, -2, -2))
	})
}

func TestSub(t *testing.T) {
	prog := makeProgram(opcode.SUB)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, int64(2), vm.estack.Pop().BigInt().Int64())
}

func TestSUBBigResult(t *testing.T) {
	prog := makeProgram(opcode.SUB)
	vm := load(prog)
	vm.estack.PushVal(getBigInt(MaxBigIntegerSizeBits, -1))
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
}

func TestSHRGood(t *testing.T) {
	prog := makeProgram(opcode.SHR)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(1), vm.estack.Pop().value)
}

func TestSHRZero(t *testing.T) {
	prog := makeProgram(opcode.SHR)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 1})
	vm.estack.PushVal(0)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem([]byte{0, 1}), vm.estack.Pop().value)
}

func TestSHRSmallValue(t *testing.T) {
	prog := makeProgram(opcode.SHR)
	vm := load(prog)
	vm.estack.PushVal(5)
	vm.estack.PushVal(minSHLArg - 1)
	checkVMFailed(t, vm)
}

func TestSHRBigArgument(t *testing.T) {
	prog := makeProgram(opcode.SHR)
	vm := load(prog)
	vm.estack.PushVal(getBigInt(MaxBigIntegerSizeBits, 0))
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestSHLGood(t *testing.T) {
	prog := makeProgram(opcode.SHL)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(16), vm.estack.Pop().value)
}

func TestSHLZero(t *testing.T) {
	prog := makeProgram(opcode.SHL)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 1})
	vm.estack.PushVal(0)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem([]byte{0, 1}), vm.estack.Pop().value)
}

func TestSHLBigValue(t *testing.T) {
	prog := makeProgram(opcode.SHL)
	vm := load(prog)
	vm.estack.PushVal(5)
	vm.estack.PushVal(maxSHLArg + 1)
	checkVMFailed(t, vm)
}

func TestSHLBigResult(t *testing.T) {
	prog := makeProgram(opcode.SHL)
	vm := load(prog)
	vm.estack.PushVal(getBigInt(MaxBigIntegerSizeBits/2, 0))
	vm.estack.PushVal(MaxBigIntegerSizeBits / 2)
	checkVMFailed(t, vm)
}

func TestSHLBigArgument(t *testing.T) {
	prog := makeProgram(opcode.SHR)
	vm := load(prog)
	vm.estack.PushVal(getBigInt(MaxBigIntegerSizeBits, 0))
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestLT(t *testing.T) {
	prog := makeProgram(opcode.LT)
	vm := load(prog)
	vm.estack.PushVal(4)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestLTE(t *testing.T) {
	prog := makeProgram(opcode.LTE)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestGT(t *testing.T) {
	prog := makeProgram(opcode.GT)
	vm := load(prog)
	vm.estack.PushVal(9)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, true, vm.estack.Pop().Bool())

}

func TestGTE(t *testing.T) {
	prog := makeProgram(opcode.GTE)
	vm := load(prog)
	vm.estack.PushVal(3)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, true, vm.estack.Pop().Bool())
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

func TestEQUALNoArguments(t *testing.T) {
	prog := makeProgram(opcode.EQUAL)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestEQUALBad1Argument(t *testing.T) {
	prog := makeProgram(opcode.EQUAL)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestEQUALGoodInteger(t *testing.T) {
	prog := makeProgram(opcode.EQUAL)
	vm := load(prog)
	vm.estack.PushVal(5)
	vm.estack.PushVal(5)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestEQUALIntegerByteArray(t *testing.T) {
	prog := makeProgram(opcode.EQUAL)
	vm := load(prog)
	vm.estack.PushVal([]byte{16})
	vm.estack.PushVal(16)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestEQUALArrayTrue(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.EQUAL)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestEQUALArrayFalse(t *testing.T) {
	prog := makeProgram(opcode.EQUAL)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	vm.estack.PushVal([]StackItem{})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestEQUALMapTrue(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.EQUAL)
	vm := load(prog)
	vm.estack.Push(&Element{value: NewMapItem()})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BoolItem{true}, vm.estack.Pop().value)
}

func TestEQUALMapFalse(t *testing.T) {
	prog := makeProgram(opcode.EQUAL)
	vm := load(prog)
	vm.estack.Push(&Element{value: NewMapItem()})
	vm.estack.Push(&Element{value: NewMapItem()})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BoolItem{false}, vm.estack.Pop().value)
}

func TestNumEqual(t *testing.T) {
	prog := makeProgram(opcode.NUMEQUAL)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestNumNotEqual(t *testing.T) {
	prog := makeProgram(opcode.NUMNOTEQUAL)
	vm := load(prog)
	vm.estack.PushVal(2)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestINC(t *testing.T) {
	prog := makeProgram(opcode.INC)
	vm := load(prog)
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, big.NewInt(2), vm.estack.Pop().BigInt())
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

func TestNEWARRAYInteger(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY)
	vm := load(prog)
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{[]StackItem{makeStackItem(false)}}, vm.estack.Pop().value)
}

func TestNEWARRAYStruct(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY)
	vm := load(prog)
	arr := []StackItem{makeStackItem(42)}
	vm.estack.Push(&Element{value: &StructItem{arr}})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{arr}, vm.estack.Pop().value)
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

	arr := makeArrayOfFalses(4)
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

func TestNEWARRAYArray(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY)
	vm := load(prog)
	arr := []StackItem{makeStackItem(42)}
	vm.estack.Push(&Element{value: &ArrayItem{arr}})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{arr}, vm.estack.Pop().value)
}

func TestNEWARRAYByteArray(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY)
	vm := load(prog)
	vm.estack.PushVal([]byte{})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{[]StackItem{}}, vm.estack.Pop().value)
}

func TestNEWARRAYBadSize(t *testing.T) {
	prog := makeProgram(opcode.NEWARRAY)
	vm := load(prog)
	vm.estack.PushVal(MaxArraySize + 1)
	checkVMFailed(t, vm)
}

func TestNEWSTRUCTInteger(t *testing.T) {
	prog := makeProgram(opcode.NEWSTRUCT)
	vm := load(prog)
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{[]StackItem{makeStackItem(false)}}, vm.estack.Pop().value)
}

func TestNEWSTRUCTArray(t *testing.T) {
	prog := makeProgram(opcode.NEWSTRUCT)
	vm := load(prog)
	arr := []StackItem{makeStackItem(42)}
	vm.estack.Push(&Element{value: &ArrayItem{arr}})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{arr}, vm.estack.Pop().value)
}

func TestNEWSTRUCTStruct(t *testing.T) {
	prog := makeProgram(opcode.NEWSTRUCT)
	vm := load(prog)
	arr := []StackItem{makeStackItem(42)}
	vm.estack.Push(&Element{value: &StructItem{arr}})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{arr}, vm.estack.Pop().value)
}

func TestNEWSTRUCTByteArray(t *testing.T) {
	prog := makeProgram(opcode.NEWSTRUCT)
	vm := load(prog)
	vm.estack.PushVal([]byte{})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{[]StackItem{}}, vm.estack.Pop().value)
}

func TestNEWSTRUCTBadSize(t *testing.T) {
	prog := makeProgram(opcode.NEWSTRUCT)
	vm := load(prog)
	vm.estack.PushVal(MaxArraySize + 1)
	checkVMFailed(t, vm)
}

func TestAPPENDArray(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.PUSH5, opcode.APPEND)
	vm := load(prog)
	vm.estack.Push(&Element{value: &ArrayItem{}})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{[]StackItem{makeStackItem(5)}}, vm.estack.Pop().value)
}

func TestAPPENDStruct(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.PUSH5, opcode.APPEND)
	vm := load(prog)
	vm.estack.Push(&Element{value: &StructItem{}})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &StructItem{[]StackItem{makeStackItem(5)}}, vm.estack.Pop().value)
}

func TestAPPENDCloneStruct(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.PUSH0, opcode.NEWSTRUCT, opcode.TOALTSTACK,
		opcode.DUPFROMALTSTACK, opcode.APPEND, opcode.FROMALTSTACK, opcode.PUSH1, opcode.APPEND)
	vm := load(prog)
	vm.estack.Push(&Element{value: &ArrayItem{}})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{[]StackItem{
		&StructItem{[]StackItem{}},
	}}, vm.estack.Pop().value)
}

func TestAPPENDBadNoArguments(t *testing.T) {
	prog := makeProgram(opcode.APPEND)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestAPPENDBad1Argument(t *testing.T) {
	prog := makeProgram(opcode.APPEND)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestAPPENDWrongType(t *testing.T) {
	prog := makeProgram(opcode.APPEND)
	vm := load(prog)
	vm.estack.PushVal([]byte{})
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
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
	vm := load(prog)
	vm.estack.PushVal(MaxArraySize)
	checkVMFailed(t, vm)
}

func TestPICKITEMBadIndex(t *testing.T) {
	prog := makeProgram(opcode.PICKITEM)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	vm.estack.PushVal(0)
	checkVMFailed(t, vm)
}

func TestPICKITEMArray(t *testing.T) {
	prog := makeProgram(opcode.PICKITEM)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{makeStackItem(1), makeStackItem(2)})
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(2), vm.estack.Pop().value)
}

func TestPICKITEMByteArray(t *testing.T) {
	prog := makeProgram(opcode.PICKITEM)
	vm := load(prog)
	vm.estack.PushVal([]byte{1, 2})
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(2), vm.estack.Pop().value)
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
	prog := makeProgram(opcode.DUP, opcode.PUSHBYTES1, 42, opcode.PICKITEM, opcode.ABS)
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
	vm := load(prog)

	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(3))
	vm.estack.Push(&Element{value: m})
	vm.estack.PushVal(makeStackItem(5))

	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(3), vm.estack.Pop().value)
}

func TestSETITEMMap(t *testing.T) {
	prog := makeProgram(opcode.SETITEM, opcode.PICKITEM)
	vm := load(prog)

	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(3))
	vm.estack.Push(&Element{value: m})
	vm.estack.PushVal(5)
	vm.estack.Push(&Element{value: m})
	vm.estack.PushVal(5)
	vm.estack.PushVal([]byte{0, 1})

	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem([]byte{0, 1}), vm.estack.Pop().value)
}

func TestSETITEMBigMapBad(t *testing.T) {
	prog := makeProgram(opcode.SETITEM)
	vm := load(prog)

	m := NewMapItem()
	for i := 0; i < MaxArraySize; i++ {
		m.Add(makeStackItem(i), makeStackItem(i))
	}
	vm.estack.Push(&Element{value: m})
	vm.estack.PushVal(MaxArraySize)
	vm.estack.PushVal(0)

	checkVMFailed(t, vm)
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

func TestSIZENoArgument(t *testing.T) {
	prog := makeProgram(opcode.SIZE)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestSIZEByteArray(t *testing.T) {
	prog := makeProgram(opcode.SIZE)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 1})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(2), vm.estack.Pop().value)
}

func TestSIZEBool(t *testing.T) {
	prog := makeProgram(opcode.SIZE)
	vm := load(prog)
	vm.estack.PushVal(false)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	// assert.Equal(t, makeStackItem(1), vm.estack.Pop().value)
	// FIXME revert when NEO 3.0 https://github.com/nspcc-dev/neo-go/issues/477
	assert.Equal(t, makeStackItem(0), vm.estack.Pop().value)
}

func TestARRAYSIZEArray(t *testing.T) {
	prog := makeProgram(opcode.ARRAYSIZE)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{
		makeStackItem(1),
		makeStackItem([]byte{}),
	})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(2), vm.estack.Pop().value)
}

func TestARRAYSIZEMap(t *testing.T) {
	prog := makeProgram(opcode.ARRAYSIZE)
	vm := load(prog)

	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(6))
	m.Add(makeStackItem([]byte{0, 1}), makeStackItem(6))
	vm.estack.Push(&Element{value: m})

	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(2), vm.estack.Pop().value)
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

func TestKEYSNoArgument(t *testing.T) {
	prog := makeProgram(opcode.KEYS)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestKEYSWrongType(t *testing.T) {
	prog := makeProgram(opcode.KEYS)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	checkVMFailed(t, vm)
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

func TestVALUESArray(t *testing.T) {
	prog := makeProgram(opcode.VALUES)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{makeStackItem(4)})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ArrayItem{[]StackItem{makeStackItem(4)}}, vm.estack.Pop().value)
}

func TestVALUESNoArgument(t *testing.T) {
	prog := makeProgram(opcode.VALUES)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestVALUESWrongType(t *testing.T) {
	prog := makeProgram(opcode.VALUES)
	vm := load(prog)
	vm.estack.PushVal(5)
	checkVMFailed(t, vm)
}

func TestHASKEYArrayTrue(t *testing.T) {
	prog := makeProgram(opcode.PUSH5, opcode.NEWARRAY, opcode.PUSH4, opcode.HASKEY)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(true), vm.estack.Pop().value)
}

func TestHASKEYArrayFalse(t *testing.T) {
	prog := makeProgram(opcode.PUSH5, opcode.NEWARRAY, opcode.PUSH5, opcode.HASKEY)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(false), vm.estack.Pop().value)
}

func TestHASKEYStructTrue(t *testing.T) {
	prog := makeProgram(opcode.PUSH5, opcode.NEWSTRUCT, opcode.PUSH4, opcode.HASKEY)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(true), vm.estack.Pop().value)
}

func TestHASKEYStructFalse(t *testing.T) {
	prog := makeProgram(opcode.PUSH5, opcode.NEWSTRUCT, opcode.PUSH5, opcode.HASKEY)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(false), vm.estack.Pop().value)
}

func TestHASKEYMapTrue(t *testing.T) {
	prog := makeProgram(opcode.HASKEY)
	vm := load(prog)
	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(6))
	vm.estack.Push(&Element{value: m})
	vm.estack.PushVal(5)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(true), vm.estack.Pop().value)
}

func TestHASKEYMapFalse(t *testing.T) {
	prog := makeProgram(opcode.HASKEY)
	vm := load(prog)
	m := NewMapItem()
	m.Add(makeStackItem(5), makeStackItem(6))
	vm.estack.Push(&Element{value: m})
	vm.estack.PushVal(6)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(false), vm.estack.Pop().value)
}

func TestHASKEYNoArguments(t *testing.T) {
	prog := makeProgram(opcode.HASKEY)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestHASKEY1Argument(t *testing.T) {
	prog := makeProgram(opcode.HASKEY)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestHASKEYWrongKeyType(t *testing.T) {
	prog := makeProgram(opcode.HASKEY)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	vm.estack.PushVal([]StackItem{})
	checkVMFailed(t, vm)
}

func TestHASKEYWrongCollectionType(t *testing.T) {
	prog := makeProgram(opcode.HASKEY)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	checkVMFailed(t, vm)
}

func TestSIGNNoArgument(t *testing.T) {
	prog := makeProgram(opcode.SIGN)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestSIGNWrongType(t *testing.T) {
	prog := makeProgram(opcode.SIGN)
	vm := load(prog)
	vm.estack.PushVal([]StackItem{})
	checkVMFailed(t, vm)
}

func TestSIGNBool(t *testing.T) {
	prog := makeProgram(opcode.SIGN)
	vm := load(prog)
	vm.estack.PushVal(false)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(0)}, vm.estack.Pop().value)
}

func TestSIGNPositiveInt(t *testing.T) {
	prog := makeProgram(opcode.SIGN)
	vm := load(prog)
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(1)}, vm.estack.Pop().value)
}

func TestSIGNNegativeInt(t *testing.T) {
	prog := makeProgram(opcode.SIGN)
	vm := load(prog)
	vm.estack.PushVal(-1)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(-1)}, vm.estack.Pop().value)
}

func TestSIGNZero(t *testing.T) {
	prog := makeProgram(opcode.SIGN)
	vm := load(prog)
	vm.estack.PushVal(0)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(0)}, vm.estack.Pop().value)
}

func TestSIGNByteArray(t *testing.T) {
	prog := makeProgram(opcode.SIGN)
	vm := load(prog)
	vm.estack.PushVal([]byte{0, 1})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &BigIntegerItem{big.NewInt(1)}, vm.estack.Pop().value)
}

func TestAppCall(t *testing.T) {
	prog := []byte{byte(opcode.APPCALL)}
	hash := util.Uint160{1, 2}
	prog = append(prog, hash.BytesBE()...)
	prog = append(prog, byte(opcode.RET))

	vm := load(prog)
	vm.SetScriptGetter(func(in util.Uint160) ([]byte, bool) {
		if in.Equals(hash) {
			return makeProgram(opcode.DEPTH), true
		}
		return nil, false
	})
	vm.estack.PushVal(2)

	runVM(t, vm)
	elem := vm.estack.Pop() // depth should be 1
	assert.Equal(t, int64(1), elem.BigInt().Int64())
}

func TestAppCallDynamicBad(t *testing.T) {
	prog := []byte{byte(opcode.APPCALL)}
	hash := util.Uint160{}
	prog = append(prog, hash.BytesBE()...)
	prog = append(prog, byte(opcode.RET))

	vm := load(prog)
	vm.SetScriptGetter(func(in util.Uint160) ([]byte, bool) {
		if in.Equals(hash) {
			return makeProgram(opcode.DEPTH), true
		}
		return nil, false
	})
	vm.estack.PushVal(2)
	vm.estack.PushVal(hash.BytesBE())

	checkVMFailed(t, vm)
}

func TestAppCallDynamicGood(t *testing.T) {
	prog := []byte{byte(opcode.APPCALL)}
	zeroHash := util.Uint160{}
	hash := util.Uint160{1, 2, 3}
	prog = append(prog, zeroHash.BytesBE()...)
	prog = append(prog, byte(opcode.RET))

	vm := load(prog)
	vm.SetScriptGetter(func(in util.Uint160) ([]byte, bool) {
		if in.Equals(hash) {
			return makeProgram(opcode.DEPTH), true
		}
		return nil, false
	})
	vm.estack.PushVal(42)
	vm.estack.PushVal(42)
	vm.estack.PushVal(hash.BytesBE())
	vm.Context().hasDynamicInvoke = true

	runVM(t, vm)
	elem := vm.estack.Pop() // depth should be 2
	assert.Equal(t, int64(2), elem.BigInt().Int64())
}

func TestSimpleCall(t *testing.T) {
	progStr := "52c56b525a7c616516006c766b00527ac46203006c766b00c3616c756653c56b6c766b00527ac46c766b51527ac46203006c766b00c36c766b51c393616c7566"
	result := 12

	prog, err := hex.DecodeString(progStr)
	require.NoError(t, err)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, result, int(vm.estack.Pop().BigInt().Int64()))
}

func TestNZtrue(t *testing.T) {
	prog := makeProgram(opcode.NZ)
	vm := load(prog)
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestNZfalse(t *testing.T) {
	prog := makeProgram(opcode.NZ)
	vm := load(prog)
	vm.estack.PushVal(0)
	runVM(t, vm)
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestPICKbadNoitem(t *testing.T) {
	prog := makeProgram(opcode.PICK)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestPICKbadNegative(t *testing.T) {
	prog := makeProgram(opcode.PICK)
	vm := load(prog)
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
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
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	checkVMFailed(t, vm)
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
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
}

func TestROLLBad2(t *testing.T) {
	prog := makeProgram(opcode.ROLL)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.estack.PushVal(3)
	checkVMFailed(t, vm)
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

func TestXTUCKbadNoitem(t *testing.T) {
	prog := makeProgram(opcode.XTUCK)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestXTUCKbadNoN(t *testing.T) {
	prog := makeProgram(opcode.XTUCK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	checkVMFailed(t, vm)
}

func TestXTUCKbadNegative(t *testing.T) {
	prog := makeProgram(opcode.XTUCK)
	vm := load(prog)
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
}

func TestXTUCKbadZero(t *testing.T) {
	prog := makeProgram(opcode.XTUCK)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(0)
	checkVMFailed(t, vm)
}

func TestXTUCKgood(t *testing.T) {
	prog := makeProgram(opcode.XTUCK)
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
	runVM(t, vm)
	assert.Equal(t, int64(topelement), vm.estack.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(topelement), vm.estack.Peek(xtuckdepth).BigInt().Int64())
}

func TestTUCKbadNoitems(t *testing.T) {
	prog := makeProgram(opcode.TUCK)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestTUCKbadNoitem(t *testing.T) {
	prog := makeProgram(opcode.TUCK)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
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

func TestOVERbadNoitem(t *testing.T) {
	prog := makeProgram(opcode.OVER)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(1), vm.estack.Pop().value)
}

func TestOVERbadNoitems(t *testing.T) {
	prog := makeProgram(opcode.OVER)
	vm := load(prog)
	checkVMFailed(t, vm)
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
	prog := makeProgram(opcode.PUSHBYTES2, 1, 0,
		opcode.PUSH1,
		opcode.OVER,
		opcode.PUSH1,
		opcode.LEFT,
		opcode.PUSHBYTES1, 2,
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
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestNIPGood(t *testing.T) {
	prog := makeProgram(opcode.NIP)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, makeStackItem(2), vm.estack.Pop().value)
}

func TestDROPBadNoItem(t *testing.T) {
	prog := makeProgram(opcode.DROP)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestDROPGood(t *testing.T) {
	prog := makeProgram(opcode.DROP)
	vm := load(prog)
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 0, vm.estack.Len())
}

func TestXDROPbadNoitem(t *testing.T) {
	prog := makeProgram(opcode.XDROP)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestXDROPbadNoN(t *testing.T) {
	prog := makeProgram(opcode.XDROP)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	checkVMFailed(t, vm)
}

func TestXDROPbadNegative(t *testing.T) {
	prog := makeProgram(opcode.XDROP)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
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

func TestINVERTbadNoitem(t *testing.T) {
	prog := makeProgram(opcode.INVERT)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestINVERTgood1(t *testing.T) {
	prog := makeProgram(opcode.INVERT)
	vm := load(prog)
	vm.estack.PushVal(0)
	runVM(t, vm)
	assert.Equal(t, int64(-1), vm.estack.Peek(0).BigInt().Int64())
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

func TestCATBadNoArgs(t *testing.T) {
	prog := makeProgram(opcode.CAT)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestCATBadOneArg(t *testing.T) {
	prog := makeProgram(opcode.CAT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abc"))
	checkVMFailed(t, vm)
}

func TestCATBadBigItem(t *testing.T) {
	prog := makeProgram(opcode.CAT)
	vm := load(prog)
	vm.estack.PushVal(make([]byte, MaxItemSize/2+1))
	vm.estack.PushVal(make([]byte, MaxItemSize/2+1))
	vm.Run()
	assert.Equal(t, true, vm.HasFailed())
}

func TestCATGood(t *testing.T) {
	prog := makeProgram(opcode.CAT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abc"))
	vm.estack.PushVal([]byte("def"))
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("abcdef"), vm.estack.Peek(0).Bytes())
}

func TestCATInt0ByteArray(t *testing.T) {
	prog := makeProgram(opcode.CAT)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal([]byte{})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ByteArrayItem{[]byte{0}}, vm.estack.Pop().value)
}

func TestCATByteArrayInt1(t *testing.T) {
	prog := makeProgram(opcode.CAT)
	vm := load(prog)
	vm.estack.PushVal([]byte{})
	vm.estack.PushVal(1)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, &ByteArrayItem{[]byte{1}}, vm.estack.Pop().value)
}

func TestSUBSTRBadNoArgs(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestSUBSTRBadOneArg(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestSUBSTRBadTwoArgs(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	vm.estack.PushVal(0)
	vm.estack.PushVal(2)
	checkVMFailed(t, vm)
}

func TestSUBSTRGood(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("bc"), vm.estack.Peek(0).Bytes())
}

func TestSUBSTRBadOffset(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(7)
	vm.estack.PushVal(1)

	// checkVMFailed(t, vm)
	// FIXME revert when NEO 3.0 https://github.com/nspcc-dev/neo-go/issues/477
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte{}, vm.estack.Peek(0).Bytes())
}

func TestSUBSTRBigLen(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(1)
	vm.estack.PushVal(6)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("bcdef"), vm.estack.Pop().Bytes())
}

func TestSUBSTRBad387(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	b := make([]byte, 6, 20)
	copy(b, "abcdef")
	vm.estack.PushVal(b)
	vm.estack.PushVal(1)
	vm.estack.PushVal(6)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("bcdef"), vm.estack.Pop().Bytes())
}

func TestSUBSTRBadNegativeOffset(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(-1)
	vm.estack.PushVal(3)
	checkVMFailed(t, vm)
}

func TestSUBSTRBadNegativeLen(t *testing.T) {
	prog := makeProgram(opcode.SUBSTR)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(3)
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
}

func TestLEFTBadNoArgs(t *testing.T) {
	prog := makeProgram(opcode.LEFT)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestLEFTBadNoString(t *testing.T) {
	prog := makeProgram(opcode.LEFT)
	vm := load(prog)
	vm.estack.PushVal(2)
	checkVMFailed(t, vm)
}

func TestLEFTBadNegativeLen(t *testing.T) {
	prog := makeProgram(opcode.LEFT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
}

func TestLEFTGood(t *testing.T) {
	prog := makeProgram(opcode.LEFT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("ab"), vm.estack.Peek(0).Bytes())
}

func TestLEFTGoodLen(t *testing.T) {
	prog := makeProgram(opcode.LEFT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(8)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("abcdef"), vm.estack.Peek(0).Bytes())
}

func TestRIGHTBadNoArgs(t *testing.T) {
	prog := makeProgram(opcode.RIGHT)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestRIGHTBadNoString(t *testing.T) {
	prog := makeProgram(opcode.RIGHT)
	vm := load(prog)
	vm.estack.PushVal(2)
	checkVMFailed(t, vm)
}

func TestRIGHTBadNegativeLen(t *testing.T) {
	prog := makeProgram(opcode.RIGHT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
}

func TestRIGHTGood(t *testing.T) {
	prog := makeProgram(opcode.RIGHT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(2)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []byte("ef"), vm.estack.Peek(0).Bytes())
}

func TestRIGHTBadLen(t *testing.T) {
	prog := makeProgram(opcode.RIGHT)
	vm := load(prog)
	vm.estack.PushVal([]byte("abcdef"))
	vm.estack.PushVal(8)
	checkVMFailed(t, vm)
}

func TestPACKBadLen(t *testing.T) {
	prog := makeProgram(opcode.PACK)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
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

func TestPACKGoodZeroLen(t *testing.T) {
	prog := makeProgram(opcode.PACK)
	vm := load(prog)
	vm.estack.PushVal(0)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, []StackItem{}, vm.estack.Peek(0).Array())
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
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
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

func TestREVERSEBadNotArray(t *testing.T) {
	prog := makeProgram(opcode.REVERSE)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func testREVERSEIssue437(t *testing.T, i1, i2 opcode.Opcode, reversed bool) {
	prog := makeProgram(
		opcode.PUSH0, i1,
		opcode.DUP, opcode.PUSH1, opcode.APPEND,
		opcode.DUP, opcode.PUSH2, opcode.APPEND,
		opcode.DUP, i2, opcode.REVERSE)
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

func TestREVERSEIssue437(t *testing.T) {
	t.Run("Array+Array", func(t *testing.T) { testREVERSEIssue437(t, opcode.NEWARRAY, opcode.NEWARRAY, true) })
	t.Run("Struct+Struct", func(t *testing.T) { testREVERSEIssue437(t, opcode.NEWSTRUCT, opcode.NEWSTRUCT, true) })
	t.Run("Array+Struct", func(t *testing.T) { testREVERSEIssue437(t, opcode.NEWARRAY, opcode.NEWSTRUCT, false) })
	t.Run("Struct+Array", func(t *testing.T) { testREVERSEIssue437(t, opcode.NEWSTRUCT, opcode.NEWARRAY, false) })
}

func TestREVERSEGoodOneElem(t *testing.T) {
	prog := makeProgram(opcode.DUP, opcode.REVERSE)
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

func TestREVERSEGoodStruct(t *testing.T) {
	eodd := []int{22, 34, 42, 55, 81}
	even := []int{22, 34, 42, 55, 81, 99}
	eall := [][]int{eodd, even}

	for _, elements := range eall {
		prog := makeProgram(opcode.DUP, opcode.REVERSE)
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

func TestREVERSEGood(t *testing.T) {
	eodd := []int{22, 34, 42, 55, 81}
	even := []int{22, 34, 42, 55, 81, 99}
	eall := [][]int{eodd, even}

	for _, elements := range eall {
		prog := makeProgram(opcode.DUP, opcode.REVERSE)
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

func TestREMOVEBadNoArgs(t *testing.T) {
	prog := makeProgram(opcode.REMOVE)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestREMOVEBadOneArg(t *testing.T) {
	prog := makeProgram(opcode.REMOVE)
	vm := load(prog)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestREMOVEBadNotArray(t *testing.T) {
	prog := makeProgram(opcode.REMOVE)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(1)
	checkVMFailed(t, vm)
}

func TestREMOVEBadIndex(t *testing.T) {
	prog := makeProgram(opcode.REMOVE)
	elements := []int{22, 34, 42, 55, 81}
	vm := load(prog)
	vm.estack.PushVal(elements)
	vm.estack.PushVal(10)
	checkVMFailed(t, vm)
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

func TestCHECKSIGNoArgs(t *testing.T) {
	prog := makeProgram(opcode.CHECKSIG)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestCHECKSIGOneArg(t *testing.T) {
	prog := makeProgram(opcode.CHECKSIG)
	pk, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	pbytes := pk.PublicKey().Bytes()
	vm := load(prog)
	vm.estack.PushVal(pbytes)
	checkVMFailed(t, vm)
}

func TestCHECKSIGNoSigLoaded(t *testing.T) {
	prog := makeProgram(opcode.CHECKSIG)
	pk, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := "NEO - An Open Network For Smart Economy"
	sig := pk.Sign([]byte(msg))
	pbytes := pk.PublicKey().Bytes()
	vm := load(prog)
	vm.estack.PushVal(sig)
	vm.estack.PushVal(pbytes)
	checkVMFailed(t, vm)
}

func TestCHECKSIGBadKey(t *testing.T) {
	prog := makeProgram(opcode.CHECKSIG)
	pk, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig := pk.Sign(msg)
	pbytes := pk.PublicKey().Bytes()[:4]
	vm := load(prog)
	vm.SetCheckedHash(hash.Sha256(msg).BytesBE())
	vm.estack.PushVal(sig)
	vm.estack.PushVal(pbytes)
	checkVMFailed(t, vm)
}

func TestCHECKSIGWrongSig(t *testing.T) {
	prog := makeProgram(opcode.CHECKSIG)
	pk, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig := pk.Sign(msg)
	pbytes := pk.PublicKey().Bytes()
	vm := load(prog)
	vm.SetCheckedHash(hash.Sha256(msg).BytesBE())
	vm.estack.PushVal(util.ArrayReverse(sig))
	vm.estack.PushVal(pbytes)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestCHECKSIGGood(t *testing.T) {
	prog := makeProgram(opcode.CHECKSIG)
	pk, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig := pk.Sign(msg)
	pbytes := pk.PublicKey().Bytes()
	vm := load(prog)
	vm.SetCheckedHash(hash.Sha256(msg).BytesBE())
	vm.estack.PushVal(sig)
	vm.estack.PushVal(pbytes)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestVERIFYGood(t *testing.T) {
	prog := makeProgram(opcode.VERIFY)
	pk, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig := pk.Sign(msg)
	pbytes := pk.PublicKey().Bytes()
	vm := load(prog)
	vm.estack.PushVal(msg)
	vm.estack.PushVal(sig)
	vm.estack.PushVal(pbytes)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, true, vm.estack.Pop().Bool())
}

func TestVERIFYBad(t *testing.T) {
	prog := makeProgram(opcode.VERIFY)
	pk, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig := pk.Sign(msg)
	pbytes := pk.PublicKey().Bytes()
	vm := load(prog)
	vm.estack.PushVal(util.ArrayReverse(msg))
	vm.estack.PushVal(sig)
	vm.estack.PushVal(pbytes)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func TestCHECKMULTISIGNoArgs(t *testing.T) {
	prog := makeProgram(opcode.CHECKMULTISIG)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestCHECKMULTISIGOneArg(t *testing.T) {
	prog := makeProgram(opcode.CHECKMULTISIG)
	pk, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	vm := load(prog)
	pbytes := pk.PublicKey().Bytes()
	vm.estack.PushVal([]StackItem{NewByteArrayItem(pbytes)})
	checkVMFailed(t, vm)
}

func TestCHECKMULTISIGNotEnoughKeys(t *testing.T) {
	prog := makeProgram(opcode.CHECKMULTISIG)
	pk1, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	pk2, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig1 := pk1.Sign(msg)
	sig2 := pk2.Sign(msg)
	pbytes1 := pk1.PublicKey().Bytes()
	vm := load(prog)
	vm.SetCheckedHash(hash.Sha256(msg).BytesBE())
	vm.estack.PushVal([]StackItem{NewByteArrayItem(sig1), NewByteArrayItem(sig2)})
	vm.estack.PushVal([]StackItem{NewByteArrayItem(pbytes1)})
	checkVMFailed(t, vm)
}

func TestCHECKMULTISIGNoHash(t *testing.T) {
	prog := makeProgram(opcode.CHECKMULTISIG)
	pk1, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	pk2, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig1 := pk1.Sign(msg)
	sig2 := pk2.Sign(msg)
	pbytes1 := pk1.PublicKey().Bytes()
	pbytes2 := pk2.PublicKey().Bytes()
	vm := load(prog)
	vm.estack.PushVal([]StackItem{NewByteArrayItem(sig1), NewByteArrayItem(sig2)})
	vm.estack.PushVal([]StackItem{NewByteArrayItem(pbytes1), NewByteArrayItem(pbytes2)})
	checkVMFailed(t, vm)
}

func TestCHECKMULTISIGBadKey(t *testing.T) {
	prog := makeProgram(opcode.CHECKMULTISIG)
	pk1, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	pk2, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig1 := pk1.Sign(msg)
	sig2 := pk2.Sign(msg)
	pbytes1 := pk1.PublicKey().Bytes()
	pbytes2 := pk2.PublicKey().Bytes()[:4]
	vm := load(prog)
	vm.SetCheckedHash(hash.Sha256(msg).BytesBE())
	vm.estack.PushVal([]StackItem{NewByteArrayItem(sig1), NewByteArrayItem(sig2)})
	vm.estack.PushVal([]StackItem{NewByteArrayItem(pbytes1), NewByteArrayItem(pbytes2)})
	checkVMFailed(t, vm)
}

func TestCHECKMULTISIGBadSig(t *testing.T) {
	prog := makeProgram(opcode.CHECKMULTISIG)
	pk1, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	pk2, err := keys.NewPrivateKey()
	assert.Nil(t, err)
	msg := []byte("NEO - An Open Network For Smart Economy")
	sig1 := pk1.Sign(msg)
	sig2 := pk2.Sign(msg)
	pbytes1 := pk1.PublicKey().Bytes()
	pbytes2 := pk2.PublicKey().Bytes()
	vm := load(prog)
	vm.SetCheckedHash(hash.Sha256(msg).BytesBE())
	vm.estack.PushVal([]StackItem{NewByteArrayItem(util.ArrayReverse(sig1)), NewByteArrayItem(sig2)})
	vm.estack.PushVal([]StackItem{NewByteArrayItem(pbytes1), NewByteArrayItem(pbytes2)})
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, false, vm.estack.Pop().Bool())
}

func initCHECKMULTISIG(msg []byte, n int) ([]StackItem, []StackItem, map[string]*keys.PublicKey, error) {
	var err error

	keyMap := make(map[string]*keys.PublicKey)
	pkeys := make([]*keys.PrivateKey, n)
	pubs := make([]StackItem, n)
	for i := range pubs {
		pkeys[i], err = keys.NewPrivateKey()
		if err != nil {
			return nil, nil, nil, err
		}

		pk := pkeys[i].PublicKey()
		data := pk.Bytes()
		pubs[i] = NewByteArrayItem(data)
		keyMap[string(data)] = pk
	}

	sigs := make([]StackItem, n)
	for i := range sigs {
		sig := pkeys[i].Sign(msg)
		sigs[i] = NewByteArrayItem(sig)
	}

	return pubs, sigs, keyMap, nil
}

func subSlice(arr []StackItem, indices []int) []StackItem {
	if indices == nil {
		return arr
	}

	result := make([]StackItem, len(indices))
	for i, j := range indices {
		result[i] = arr[j]
	}

	return result
}

func initCHECKMULTISIGVM(t *testing.T, n int, ik, is []int) *VM {
	prog := makeProgram(opcode.CHECKMULTISIG)
	v := load(prog)
	msg := []byte("NEO - An Open Network For Smart Economy")

	v.SetCheckedHash(hash.Sha256(msg).BytesBE())

	pubs, sigs, _, err := initCHECKMULTISIG(msg, n)
	require.NoError(t, err)

	pubs = subSlice(pubs, ik)
	sigs = subSlice(sigs, is)

	v.estack.PushVal(sigs)
	v.estack.PushVal(pubs)

	return v
}

func testCHECKMULTISIGGood(t *testing.T, n int, is []int) {
	v := initCHECKMULTISIGVM(t, n, nil, is)

	runVM(t, v)
	assert.Equal(t, 1, v.estack.Len())
	assert.True(t, v.estack.Pop().Bool())
}

func TestCHECKMULTISIGGood(t *testing.T) {
	t.Run("3_1", func(t *testing.T) { testCHECKMULTISIGGood(t, 3, []int{1}) })
	t.Run("2_2", func(t *testing.T) { testCHECKMULTISIGGood(t, 2, []int{0, 1}) })
	t.Run("3_3", func(t *testing.T) { testCHECKMULTISIGGood(t, 3, []int{0, 1, 2}) })
	t.Run("3_2", func(t *testing.T) { testCHECKMULTISIGGood(t, 3, []int{0, 2}) })
	t.Run("4_2", func(t *testing.T) { testCHECKMULTISIGGood(t, 4, []int{0, 2}) })
	t.Run("10_7", func(t *testing.T) { testCHECKMULTISIGGood(t, 10, []int{2, 3, 4, 5, 6, 8, 9}) })
	t.Run("12_9", func(t *testing.T) { testCHECKMULTISIGGood(t, 12, []int{0, 1, 4, 5, 6, 7, 8, 9}) })
}

func testCHECKMULTISIGBad(t *testing.T, n int, ik, is []int) {
	v := initCHECKMULTISIGVM(t, n, ik, is)

	runVM(t, v)
	assert.Equal(t, 1, v.estack.Len())
	assert.False(t, v.estack.Pop().Bool())
}

func TestCHECKMULTISIGBad(t *testing.T) {
	t.Run("1_1 wrong signature", func(t *testing.T) { testCHECKMULTISIGBad(t, 2, []int{0}, []int{1}) })
	t.Run("3_2 wrong order", func(t *testing.T) { testCHECKMULTISIGBad(t, 3, []int{0, 2}, []int{2, 0}) })
	t.Run("3_2 duplicate sig", func(t *testing.T) { testCHECKMULTISIGBad(t, 3, nil, []int{0, 0}) })
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

func TestSWAPBad1(t *testing.T) {
	prog := makeProgram(opcode.SWAP)
	vm := load(prog)
	vm.estack.PushVal(4)
	checkVMFailed(t, vm)
}

func TestSWAPBad2(t *testing.T) {
	prog := makeProgram(opcode.SWAP)
	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestXSWAPGood(t *testing.T) {
	prog := makeProgram(opcode.XSWAP)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.estack.PushVal(4)
	vm.estack.PushVal(5)
	vm.estack.PushVal(3)
	runVM(t, vm)
	assert.Equal(t, 5, vm.estack.Len())
	assert.Equal(t, int64(2), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(4), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(3), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(5), vm.estack.Pop().BigInt().Int64())
	assert.Equal(t, int64(1), vm.estack.Pop().BigInt().Int64())
}

func TestXSWAPBad1(t *testing.T) {
	prog := makeProgram(opcode.XSWAP)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(-1)
	checkVMFailed(t, vm)
}

func TestXSWAPBad2(t *testing.T) {
	prog := makeProgram(opcode.XSWAP)
	vm := load(prog)
	vm.estack.PushVal(1)
	vm.estack.PushVal(2)
	vm.estack.PushVal(3)
	vm.estack.PushVal(4)
	vm.estack.PushVal(4)
	checkVMFailed(t, vm)
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
	prog := makeProgram(opcode.PUSHBYTES2, 1, 0,
		opcode.DUP,
		opcode.PUSH1,
		opcode.LEFT,
		opcode.PUSHBYTES1, 2,
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

func TestSHA1(t *testing.T) {
	// 0x0100 hashes to 0e356ba505631fbf715758bed27d503f8b260e3a
	res := "0e356ba505631fbf715758bed27d503f8b260e3a"
	prog := makeProgram(opcode.PUSHBYTES2, 1, 0,
		opcode.SHA1)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, res, hex.EncodeToString(vm.estack.Pop().Bytes()))
}

func TestSHA256(t *testing.T) {
	// 0x0100 hashes to 47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254
	res := "47dc540c94ceb704a23875c11273e16bb0b8a87aed84de911f2133568115f254"
	prog := makeProgram(opcode.PUSHBYTES2, 1, 0,
		opcode.SHA256)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, res, hex.EncodeToString(vm.estack.Pop().Bytes()))
}

func TestHASH160(t *testing.T) {
	// 0x0100 hashes to fbc22d517f38e7612798ece8e5957cf6c41d8caf
	res := "fbc22d517f38e7612798ece8e5957cf6c41d8caf"
	prog := makeProgram(opcode.PUSHBYTES2, 1, 0,
		opcode.HASH160)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, res, hex.EncodeToString(vm.estack.Pop().Bytes()))
}

func TestHASH256(t *testing.T) {
	// 0x0100 hashes to 677b2d718464ee0121475600b929c0b4155667486577d1320b18c2dc7d4b4f99
	res := "677b2d718464ee0121475600b929c0b4155667486577d1320b18c2dc7d4b4f99"
	prog := makeProgram(opcode.PUSHBYTES2, 1, 0,
		opcode.HASH256)
	vm := load(prog)
	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, res, hex.EncodeToString(vm.estack.Pop().Bytes()))
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

var stackIsolationTestCases = map[opcode.Opcode][]struct {
	name   string
	params []interface{}
}{
	opcode.CALLE: {
		{
			name: "no_params",
		},
		{
			name:   "with_params",
			params: []interface{}{big.NewInt(5), true, []byte{1, 2, 3}},
		},
	},
	opcode.CALLED: {
		{
			name: "no_paranms",
		},
		{
			name:   "with_params",
			params: []interface{}{big.NewInt(5), true, []byte{1, 2, 3}},
		},
	},
	opcode.CALLET: {
		{
			name: "no_params",
		},
		{
			name:   "with_params",
			params: []interface{}{big.NewInt(5), true, []byte{1, 2, 3}},
		},
	},
	opcode.CALLEDT: {
		{
			name: "no_params",
		},
		{
			name:   "with_params",
			params: []interface{}{big.NewInt(5), true, []byte{1, 2, 3}},
		},
	},
}

func TestStackIsolationOpcodes(t *testing.T) {
	scriptHash := util.Uint160{1, 2, 3}
	for code, codeTestCases := range stackIsolationTestCases {
		t.Run(code.String(), func(t *testing.T) {
			for _, testCase := range codeTestCases {
				t.Run(testCase.name, func(t *testing.T) {
					rvcount := len(testCase.params) + 1 // parameters + DEPTH
					pcount := len(testCase.params)
					prog := []byte{byte(code), byte(rvcount), byte(pcount)}
					if code == opcode.CALLE || code == opcode.CALLET {
						prog = append(prog, scriptHash.BytesBE()...)
					}
					prog = append(prog, byte(opcode.RET))

					vm := load(prog)
					vm.SetScriptGetter(func(in util.Uint160) ([]byte, bool) {
						if in.Equals(scriptHash) {
							return makeProgram(opcode.DEPTH), true
						}
						return nil, false
					})
					if code == opcode.CALLED || code == opcode.CALLEDT {
						vm.Context().hasDynamicInvoke = true
					}
					if code == opcode.CALLET || code == opcode.CALLEDT {
						vm.Context().rvcount = rvcount
					}
					// add extra parameter just to check that it won't appear on new estack
					vm.estack.PushVal(7)
					for _, param := range testCase.params {
						vm.estack.PushVal(param)
					}
					if code == opcode.CALLED || code == opcode.CALLEDT {
						vm.estack.PushVal(scriptHash.BytesBE())
					}

					runVM(t, vm)
					elem := vm.estack.Pop() // depth should be equal to params count, as it's a separate execution context
					assert.Equal(t, int64(pcount), elem.BigInt().Int64())
					for i := pcount; i > 0; i-- {
						p := vm.estack.Pop()
						assert.Equal(t, testCase.params[i-1], p.value.Value())
					}
				})
			}
		})
	}
}

func TestCALLIAltStack(t *testing.T) {
	prog := []byte{
		byte(opcode.PUSH1),
		byte(opcode.DUP),
		byte(opcode.TOALTSTACK),
		byte(opcode.JMP), 0x5, 0, // to CALLI (+5 including  JMP parameters)
		byte(opcode.FROMALTSTACK), // altstack is empty, so panic here
		byte(opcode.RET),
		byte(opcode.CALLI), byte(0), byte(0), 0xfc, 0xff, // to FROMALTSTACK (-4) with clean stacks
		byte(opcode.RET),
	}

	vm := load(prog)
	checkVMFailed(t, vm)
}

func TestCALLIDEPTH(t *testing.T) {
	prog := []byte{
		byte(opcode.PUSH3),
		byte(opcode.PUSH2),
		byte(opcode.PUSH1),
		byte(opcode.JMP), 0x5, 0, // to CALLI (+5 including JMP parameters)
		byte(opcode.DEPTH), // depth is 2, put it on stack
		byte(opcode.RET),
		byte(opcode.CALLI), byte(3), byte(2), 0xfc, 0xff, // to DEPTH (-4) with [21 on stack
		byte(opcode.RET),
	}

	vm := load(prog)

	runVM(t, vm)
	assert.Equal(t, 4, vm.estack.Len())
	assert.Equal(t, int64(2), vm.estack.Pop().BigInt().Int64()) // depth was 2
	for i := 1; i < 4; i++ {
		assert.Equal(t, int64(i), vm.estack.Pop().BigInt().Int64())
	}
}

func TestCALLI(t *testing.T) {
	prog := []byte{
		byte(opcode.PUSH3),
		byte(opcode.PUSH2),
		byte(opcode.PUSH1),
		byte(opcode.JMP), 0x5, 0, // to CALLI (+5 including JMP parameters)
		byte(opcode.SUB), // 2 - 1
		byte(opcode.RET),
		byte(opcode.CALLI), byte(1), byte(2), 0xfc, 0xff, // to SUB (-4) with [21 on stack
		byte(opcode.MUL), // 3 * 1
		byte(opcode.RET),
	}

	vm := load(prog)

	runVM(t, vm)
	assert.Equal(t, 1, vm.estack.Len())
	assert.Equal(t, int64(3), vm.estack.Pop().BigInt().Int64())
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
