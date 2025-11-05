package vm

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func benchOpcodeInt(t *testing.B, f func() *VM, fail bool) {
	for t.Loop() {
		t.StopTimer()
		v := f()
		t.StartTimer()
		err := v.Step()
		t.StopTimer()
		if fail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		t.StartTimer()
	}
}

func benchOpcode(t *testing.B, f func() *VM) {
	benchOpcodeInt(t, f, false)
}

func opVM(op opcode.Opcode) func() *VM {
	return opParamVM(op, nil)
}

func opParamVM(op opcode.Opcode, param []byte) func() *VM {
	return opParamPushVM(op, param)
}

func opParamPushVM(op opcode.Opcode, param []byte, items ...any) func() *VM {
	return opParamSlotsPushVM(op, param, 0, 0, 0, items...)
}

func opParamSlotsPushVM(op opcode.Opcode, param []byte, sslot int, slotloc int, slotarg int, items ...any) func() *VM {
	return opParamSlotsPushReadOnlyVM(op, param, sslot, slotloc, slotarg, true, items...)
}

func opParamSlotsPushReadOnlyVM(op opcode.Opcode, param []byte, sslot int, slotloc int, slotarg int, isReadOnly bool, items ...any) func() *VM {
	return func() *VM {
		script := []byte{byte(op)}
		script = append(script, param...)
		v := load(script)
		v.SyscallHandler = func(_ *VM, _ uint32) error {
			return nil
		}
		if sslot != 0 {
			v.Context().sc.static.init(sslot, &v.refs)
		}
		if slotloc != 0 && slotarg != 0 {
			v.Context().local.init(slotloc, &v.refs)
			v.Context().arguments.init(slotarg, &v.refs)
		}
		for i := range items {
			item, ok := items[i].(stackitem.Item)
			if ok && isReadOnly {
				item = stackitem.DeepCopy(item, true)
			} else {
				item = stackitem.Make(items[i])
			}
			v.estack.PushVal(item)
		}
		return v
	}
}

func exceptParamPushVM(op opcode.Opcode, param []byte, items ...any) func() *VM {
	return func() *VM {
		v := opParamPushVM(op, param, items...)()
		v.Context().tryStack.PushVal(newExceptionHandlingContext(0, 0))
		return v
	}
}

const countPoints = 8

func getBigInts(minBits, maxBits, num int) []*big.Int {
	step := uint((maxBits - minBits) / num)
	curr := step - 1
	res := make([]*big.Int, 0, num)
	for range num {
		res = append(res, new(big.Int).Lsh(big.NewInt(1), curr))
		curr += step
	}
	return res
}

func getArrayWithDeep(d int) []stackitem.Item {
	var item stackitem.Item = stackitem.Null{}
	for range d - 1 {
		item = stackitem.NewArray([]stackitem.Item{item})
	}
	return []stackitem.Item{item}
}

func getStructWithDeep(d int) []stackitem.Item {
	var item stackitem.Item = stackitem.Null{}
	for range d - 1 {
		item = stackitem.NewStruct([]stackitem.Item{item})
	}
	return []stackitem.Item{item}
}

func getUniqueKeys(num, keySize int) [][]byte {
	keys := make([][]byte, 0, num)
	for i := range num {
		key := make([]byte, keySize)
		j := keySize - 1
		for i > 0 {
			key[j] = byte(i & 0xff)
			i >>= 8
			j--
		}
		keys = append(keys, key)
	}
	return keys
}

func BenchmarkIs(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
		items  []any
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.ISNULL, opcode.ISTYPE} {
		currDeep := stepDeep - 1
		for range countPoints {
			testCases = append(testCases, testCase{
				name:   fmt.Sprintf("%s %d", op, currDeep),
				op:     op,
				params: []byte{0},
				items:  []any{stackitem.NewArray(getArrayWithDeep(currDeep))},
			})
			currDeep += stepDeep
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, tc.params, tc.items...))
		})
	}
}

func BenchmarkPushInt(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
	}
	var (
		testCases    []testCase
		opToNumBytes = map[opcode.Opcode]int{
			opcode.PUSHINT8:   1,
			opcode.PUSHINT16:  2,
			opcode.PUSHINT32:  4,
			opcode.PUSHINT64:  8,
			opcode.PUSHINT128: 16,
			opcode.PUSHINT256: 32,
		}
	)
	for op, numBytes := range opToNumBytes {
		buf := make([]byte, numBytes)
		buf[numBytes-1] = 0x80
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %d", op, numBytes),
			op:     op,
			params: buf,
		})
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamVM(tc.op, tc.params))
		})
	}
}

func BenchmarkPushData(b *testing.B) {
	type testCase struct {
		name   string
		params []byte
	}
	var (
		testCases []testCase
		stepLen   = stackitem.MaxSize / countPoints
		currLen   = stepLen
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %d", opcode.PUSHDATA4, currLen),
			params: make([]byte, currLen),
		})
		currLen += stepLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamVM(opcode.PUSHDATA4, tc.params))
		})
	}
}

func BenchmarkConvert(b *testing.B) {
	type testCase struct {
		name   string
		params []byte
		items  []any
	}
	var (
		testCases        []testCase
		stepArrayLen     = MaxStackSize / countPoints
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currArrayLen     = stepArrayLen - 1
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %s %s %d", opcode.CONVERT, stackitem.ArrayT, stackitem.StructT, currArrayLen),
			params: []byte{byte(stackitem.StructT)},
			items:  []any{make([]any, currArrayLen)},
		})
		currArrayLen += stepArrayLen
	}
	for range countPoints {
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %s %s %d", opcode.CONVERT, stackitem.ByteArrayT, stackitem.BufferT, currByteArrayLen),
			params: []byte{byte(stackitem.BufferT)},
			items:  []any{stackitem.NewByteArray(make([]byte, currByteArrayLen))},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.CONVERT, tc.params, tc.items...))
		})
	}
}

func BenchmarkInitSlot(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
		items  []any
	}
	var (
		testCases []testCase
		stepSlots = byte(math.MaxUint8 / countPoints)
		currSlots = stepSlots
	)
	// TODO: do not ignore the len and depth of the argument
	for range countPoints {
		for _, params := range []struct{ localSlots, argSlots byte }{{currSlots, 0}, {0, currSlots}} {
			testCases = append(testCases, testCase{
				name:   fmt.Sprintf("%s %d %d", opcode.INITSLOT, params.localSlots, params.argSlots),
				op:     opcode.INITSLOT,
				params: []byte{params.localSlots, params.argSlots},
				items:  make([]any, params.argSlots),
			})
		}
		testCases = append(testCases, testCase{
			name:   fmt.Sprintf("%s %d", opcode.INITSSLOT, currSlots),
			op:     opcode.INITSSLOT,
			params: []byte{currSlots},
		})
		currSlots += stepSlots
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, tc.params, tc.items...))
		})
	}
}

func BenchmarkLD(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		currDeep  = stepDeep - 1
	)
	for range countPoints {
		deep := currDeep
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("%s %d", opcode.LDSFLD0, currDeep),
			f: func() *VM {
				item := stackitem.NewArray(getArrayWithDeep(deep))
				v := opVM(opcode.LDSFLD0)()
				slot := []stackitem.Item{item}
				v.Context().sc.static = slot
				return v
			},
		})
		currDeep += stepDeep
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkST(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		currDeep  = stepDeep - 1
	)
	for range countPoints {
		for _, params := range []struct{ removedDeep, addedDeep int }{
			{currDeep, 1},
			{1, currDeep},
		} {
			removedDeep := params.removedDeep
			addedDeep := params.addedDeep
			testCases = append(testCases, testCase{
				name: fmt.Sprintf("%s %d %d", opcode.STSFLD0, params.removedDeep, params.addedDeep),
				f: func() *VM {
					item := stackitem.NewArray(getArrayWithDeep(addedDeep))
					v := opParamPushVM(opcode.STSFLD0, nil, item)()
					slot := []stackitem.Item{stackitem.NewArray(getArrayWithDeep(removedDeep))}
					v.Context().sc.static = slot
					v.refs.Add(slot[0])
					return v
				},
			})
		}
		currDeep += stepDeep
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkNew(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
		items  []any
	}
	var (
		testCases        []testCase
		stepArrayLen     = MaxStackSize / countPoints
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currArrayLen     = stepArrayLen - 1
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		for _, typ := range []stackitem.Type{stackitem.BooleanT, stackitem.IntegerT, stackitem.ByteArrayT, stackitem.AnyT} {
			testCases = append(testCases, testCase{
				name:   fmt.Sprintf("%s %s %d", opcode.NEWARRAYT, typ, currArrayLen),
				op:     opcode.NEWARRAYT,
				params: []byte{byte(typ)},
				items:  []any{currArrayLen},
			})
		}
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %d", opcode.NEWARRAY, currArrayLen),
			op:    opcode.NEWARRAY,
			items: []any{currArrayLen},
		})
		currArrayLen += stepArrayLen
	}
	for range countPoints {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %d", opcode.NEWBUFFER, currByteArrayLen),
			op:    opcode.NEWBUFFER,
			items: []any{currByteArrayLen},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, tc.params, tc.items...))
		})
	}
}

func BenchmarkMemCpy(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases        []testCase
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		buf1 := make([]byte, currByteArrayLen)
		buf2 := make([]byte, currByteArrayLen)
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %d", opcode.MEMCPY, currByteArrayLen),
			items: []any{stackitem.NewBuffer(buf1), 0, stackitem.NewBuffer(buf2), 0, currByteArrayLen},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.MEMCPY, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkCat(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases        []testCase
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("%s %d", opcode.CAT, currByteArrayLen),
			items: []any{
				[]byte{},
				make([]byte, currByteArrayLen),
			},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.CAT, nil, tc.items...))
		})
	}
}

func BenchmarkSubstr(b *testing.B) {
	type testCase struct {
		op    opcode.Opcode
		name  string
		items []any
	}
	var (
		testCases        []testCase
		stepByteArrayLen = stackitem.MaxSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.SUBSTR, opcode.LEFT} {
		currByteArrayLen := stepByteArrayLen
		for range countPoints {
			items := []any{make([]byte, currByteArrayLen)}
			if op == opcode.SUBSTR {
				items = append(items, 0)
			}
			items = append(items, currByteArrayLen)
			testCases = append(testCases, testCase{
				op:    op,
				name:  fmt.Sprintf("%s %d", op, currByteArrayLen),
				items: items,
			})
			currByteArrayLen += stepByteArrayLen
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkDrop(b *testing.B) {
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	itemsForOpcode := func(op opcode.Opcode, arr []stackitem.Item) []any {
		var items []any
		switch op {
		case opcode.DROP, opcode.CLEAR:
			items = []any{arr}
		case opcode.NIP, opcode.XDROP:
			items = []any{arr, 0}
		}
		return items
	}
	var (
		testCases     []testCase
		stepDeep      = MaxStackSize / countPoints
		stepStackSize = MaxStackSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.DROP, opcode.NIP, opcode.XDROP, opcode.CLEAR} {
		currDeep := stepDeep
		for range countPoints {
			name := fmt.Sprintf("%s %d", op, currDeep)
			if op == opcode.XDROP {
				name += " 0"
			} else if op == opcode.CLEAR {
				name += " 1"
			}
			testCases = append(testCases, testCase{
				name:  name,
				op:    op,
				items: itemsForOpcode(op, getArrayWithDeep(currDeep)),
			})
			currDeep += stepDeep
		}
	}
	for _, op := range []opcode.Opcode{opcode.XDROP, opcode.CLEAR} {
		currStackSize := stepStackSize
		for range countPoints {
			items := make([]any, currStackSize)
			items[currStackSize-1] = currStackSize - 2
			name := fmt.Sprintf("%s %d %d", op, 1, currStackSize-2)
			if op == opcode.CLEAR {
				name = fmt.Sprintf("%s %d %d", op, currStackSize, currStackSize)
			}
			testCases = append(testCases, testCase{
				name:  name,
				op:    op,
				items: items,
			})
			currStackSize += stepStackSize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkDup(b *testing.B) {
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	itemsForOpcode := func(op opcode.Opcode, item any) []any {
		var items []any
		switch op {
		case opcode.TUCK:
			items = []any{nil, item}
		case opcode.DUP:
			items = []any{item}
		case opcode.OVER, opcode.PICK:
			items = []any{item, 0}
		}
		return items
	}
	var (
		testCases        []testCase
		stepByteArrayLen = stackitem.MaxSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.DUP, opcode.OVER, opcode.TUCK, opcode.PICK} {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s", op, stackitem.AnyT),
			op:    op,
			items: itemsForOpcode(op, nil),
		})
		currByteArrayLen := stepByteArrayLen
		for range countPoints {
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d", op, stackitem.ByteArrayT, currByteArrayLen),
				op:    op,
				items: itemsForOpcode(op, make([]byte, currByteArrayLen)),
			})
			currByteArrayLen += stepByteArrayLen
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkRollAndReverse(b *testing.B) {
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	var (
		testCases     []testCase
		stepStackSize = MaxStackSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.ROLL, opcode.REVERSEN} {
		currStackSize := stepStackSize
		for range countPoints {
			var (
				items = make([]any, currStackSize)
				n     = currStackSize - 2
			)
			if op == opcode.REVERSEN {
				n++
			}
			items[currStackSize-1] = n
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, n),
				op:    op,
				items: items,
			})
			currStackSize += stepStackSize
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkIntegerOperands(b *testing.B) {
	unaryOpcodes := []opcode.Opcode{
		opcode.INVERT,
		opcode.SIGN,
		opcode.ABS,
		opcode.NEGATE,
		opcode.INC,
		opcode.DEC,
		opcode.SQRT,
		opcode.NZ,
	}
	binaryOpcodes := []opcode.Opcode{
		opcode.AND,
		opcode.OR,
		opcode.XOR,
		opcode.EQUAL,
		opcode.NOTEQUAL,
		opcode.ADD,
		opcode.SUB,
		opcode.DIV,
		opcode.MOD,
		opcode.NUMEQUAL,
		opcode.NUMNOTEQUAL,
		opcode.LT,
		opcode.LE,
		opcode.GT,
		opcode.GE,
		opcode.MIN,
		opcode.MAX,
	}
	ternaryOpcodes := []opcode.Opcode{
		opcode.MODMUL,
		opcode.MODPOW,
	}
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	var (
		testCases   []testCase
		stepNumBits = (stackitem.MaxBigIntegerSizeBits - 1) / countPoints
		bigInts1    = getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
	)
	for _, op := range unaryOpcodes {
		for i := range bigInts1 {
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i]},
			})
		}
	}
	bigInts2 := getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
	for _, op := range binaryOpcodes {
		for i := range bigInts1 {
			n := bigInts2[i]
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i], n},
			})
		}
	}
	bigInts3 := getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
	for _, op := range ternaryOpcodes {
		for i := range bigInts1 {
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i], bigInts2[i], bigInts3[i]},
			})
		}
	}
	mulBigInts := getBigInts(0, (stackitem.MaxBigIntegerSizeBits/2)-1, countPoints)
	for _, n := range mulBigInts {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %d", opcode.MUL, n.BitLen()),
			op:    opcode.MUL,
			items: []any{n, n},
		})
	}
	for _, op := range []opcode.Opcode{opcode.POW, opcode.SHL, opcode.SHR} {
		currNumBits := stepNumBits
		for range countPoints {
			items := []any{new(big.Int).Lsh(big.NewInt(1), uint(currNumBits))}
			switch op {
			case opcode.POW:
				items = append(items, int(math.Floor(float64(stackitem.MaxBigIntegerSizeBits)/float64(currNumBits))))
			case opcode.SHL:
				items = append(items, stackitem.MaxBigIntegerSizeBits-currNumBits-2)
			case opcode.SHR:
				items = append(items, currNumBits)
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, currNumBits),
				op:    op,
				items: items,
			})
			currNumBits += stepNumBits
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkAppend(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.StructT} {
		currDeep := stepDeep - 1
		for range countPoints {
			var item stackitem.Item
			if typ == stackitem.ArrayT {
				item = stackitem.NewArray(getArrayWithDeep(currDeep))
			} else {
				item = stackitem.NewStruct(getStructWithDeep(currDeep))
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %s %d", opcode.APPEND, stackitem.ArrayT, typ, currDeep),
				items: []any{[]any{}, item},
			})
			currDeep += stepDeep
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.APPEND, nil, tc.items...))
		})
	}
}

func BenchmarkPackMap(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		stepN     = MaxStackSize / (2 * countPoints)
		currDeep  = stepDeep - 1
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d %d", opcode.PACKMAP, stackitem.AnyT, currDeep, 1),
			items: []any{stackitem.NewArray(getArrayWithDeep(currDeep - 2)), true, 1},
		})
		currDeep += stepDeep
	}
	for _, typ := range []stackitem.Type{stackitem.IntegerT, stackitem.ByteArrayT} {
		currN := stepN - 1
		for range countPoints {
			items := make([]any, 0, 2*currN)
			if typ == stackitem.ByteArrayT {
				keys := getUniqueKeys(currN, stackitem.MaxKeySize)
				for _, key := range keys {
					items = append(items, nil, key)
				}
			} else {
				for i := range 2 * currN {
					items = append(items, i)
				}
			}
			items = append(items, currN)
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.PACKMAP, typ, 2*currN, currN),
				items: items,
			})
			currN += stepN
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.PACKMAP, nil, tc.items...))
		})
	}
}

func BenchmarkPack(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		stepN     = MaxStackSize / countPoints
		currDeep  = stepDeep - 1
		currN     = stepN - 1
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %d %d", opcode.PACK, currDeep, 1),
			items: []any{stackitem.NewArray(getArrayWithDeep(currDeep - 1)), 1},
		})
		currDeep += stepDeep
	}
	for range countPoints {
		items := make([]any, currN)
		items = append(items, currN)
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %d %d", opcode.PACK, currN, currN),
			items: items,
		})
		currN += stepN
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.PACK, nil, tc.items...))
		})
	}
}

func BenchmarkUnpack(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		stepN     = MaxStackSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDeep := stepDeep - 1
		for range countPoints {
			var item stackitem.Item
			if typ == stackitem.ArrayT {
				item = stackitem.NewArray(getArrayWithDeep(currDeep))
			} else {
				item = stackitem.NewMapWithValue([]stackitem.MapElement{{
					Key:   stackitem.NewBool(true),
					Value: stackitem.NewArray(getArrayWithDeep(currDeep - 2)),
				}})
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.UNPACK, typ, currDeep, 1),
				items: []any{item},
			})
			currDeep += stepDeep
		}
		currN := stepN - 1
		for range countPoints {
			var item stackitem.Item
			if typ == stackitem.ArrayT {
				item = stackitem.NewArray(make([]stackitem.Item, currN))
			} else {
				var (
					keys = getUniqueKeys(currN/2, stackitem.MaxKeySize)
					m    = make([]stackitem.MapElement, 0, currN/2)
				)
				for _, key := range keys {
					m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
				}
				item = stackitem.NewMapWithValue(m)
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.UNPACK, typ, currN, currN),
				items: []any{item},
			})
			currN += stepN
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.UNPACK, nil, tc.items...))
		})
	}
}

func BenchmarkPickItem(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases        []testCase
		stepDeep         = MaxStackSize / countPoints
		stepN            = MaxStackSize / (2 * countPoints)
		stepByteArrayLen = stackitem.MaxSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDeep := stepDeep - 1
		for range countPoints {
			var item stackitem.Item
			if typ == stackitem.ArrayT {
				item = stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray(getArrayWithDeep(currDeep)),
				})
			} else {
				item = stackitem.NewMapWithValue([]stackitem.MapElement{{
					Key:   stackitem.NewBigInteger(big.NewInt(0)),
					Value: stackitem.NewArray(getArrayWithDeep(currDeep)),
				}})
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d %s", opcode.PICKITEM, typ, currDeep, 1, stackitem.ArrayT),
				items: []any{item, 0},
			})
			currDeep += stepDeep
		}
		if typ == stackitem.MapT {
			currN := stepN - 1
			for range countPoints {
				keys := getUniqueKeys(currN, stackitem.MaxKeySize)
				m := make([]stackitem.MapElement, 0, currN)
				for _, key := range keys {
					m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
				}
				m[currN-1].Value = stackitem.Null{}
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d %s", opcode.PICKITEM, typ, 1, currN, stackitem.AnyT),
					items: []any{stackitem.NewMapWithValue(m), keys[currN-1]},
				})
				currN += stepN
			}
		}
		currByteArrayLen := stepByteArrayLen - 1
		for range countPoints {
			var item stackitem.Item
			if typ == stackitem.ArrayT {
				item = stackitem.NewArray([]stackitem.Item{
					stackitem.NewByteArray(make([]byte, currByteArrayLen)),
				})
			} else {
				item = stackitem.NewMapWithValue([]stackitem.MapElement{
					{
						Key:   stackitem.NewBigInteger(big.NewInt(0)),
						Value: stackitem.NewByteArray(make([]byte, currByteArrayLen)),
					},
				})
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %d %s %d", opcode.PICKITEM, typ, 1, 1, stackitem.ByteArrayT, currByteArrayLen),
				items: []any{item, 0},
			})
			currByteArrayLen += stepByteArrayLen
		}
	}
	testCases = append(testCases, testCase{
		name:  fmt.Sprintf("%s %s %d %d %s", opcode.PICKITEM, stackitem.ByteArrayT, 1, 1, stackitem.IntegerT),
		items: []any{[]byte{42}, 0},
	})
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.PICKITEM, nil, tc.items...))
		})
	}
}

func BenchmarkSetItem(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		stepN     = MaxStackSize / (2 * countPoints)
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDeep := stepDeep - 1
		for range countPoints {
			for _, params := range []struct{ removedDeep, addedDeep int }{
				{currDeep, 1},
				{1, currDeep},
			} {
				removedDeep := params.removedDeep
				addedDeep := params.addedDeep
				testCases = append(testCases, testCase{
					name: fmt.Sprintf("%s %s %d %d %d", opcode.SETITEM, typ, removedDeep, addedDeep, 1),
					f: func() *VM {
						var item, addedItem stackitem.Item
						if typ == stackitem.ArrayT {
							item = stackitem.NewArray(getArrayWithDeep(removedDeep))
							addedItem = stackitem.NewArray(getArrayWithDeep(addedDeep - 1))
						} else {
							item = stackitem.NewMapWithValue([]stackitem.MapElement{
								{
									Key:   stackitem.NewBigInteger(big.NewInt(0)),
									Value: stackitem.NewArray(getArrayWithDeep(removedDeep - 2)),
								},
							})
							addedItem = stackitem.NewArray(getArrayWithDeep(addedDeep - 2))
						}
						v := opVM(opcode.SETITEM)()
						v.estack.PushVal(item)
						v.estack.PushVal(0)
						v.estack.PushVal(addedItem)
						return v
					},
				})
			}
			currDeep += stepDeep
		}
		if typ == stackitem.MapT {
			currN := stepN - 1
			for range countPoints {
				n := currN
				testCases = append(testCases, testCase{
					name: fmt.Sprintf("%s %s %d %d %d", opcode.SETITEM, typ, 1, 1, currN),
					f: func() *VM {
						keys := getUniqueKeys(n, stackitem.MaxKeySize)
						m := make([]stackitem.MapElement, 0, n)
						for _, key := range keys {
							m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
						}
						m[n-1].Value = stackitem.Null{}
						v := opVM(opcode.SETITEM)()
						v.estack.PushVal(stackitem.NewMapWithValue(m))
						v.estack.PushVal(keys[n-1])
						v.estack.PushVal(stackitem.Null{})
						return v
					},
				})
				currN += stepN
			}
		}
	}
	testCases = append(testCases, testCase{
		name: fmt.Sprintf("%s %s", opcode.SETITEM, stackitem.BufferT),
		f: func() *VM {
			v := opVM(opcode.SETITEM)()
			v.estack.PushVal(stackitem.NewBuffer([]byte{42}))
			v.estack.PushVal(0)
			v.estack.PushVal(42)
			return v
		},
	})
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkReverseItems(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases        []testCase
		stepArrayLen     = MaxStackSize / countPoints
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currArrayLen     = stepArrayLen
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d", opcode.REVERSEITEMS, stackitem.ArrayT, currArrayLen),
			items: []any{make([]any, currArrayLen)},
		})
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d", opcode.REVERSEITEMS, stackitem.BufferT, currByteArrayLen),
			items: []any{make([]any, currByteArrayLen)},
		})
		currArrayLen += stepArrayLen
		currByteArrayLen += stepByteArrayLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.REVERSEITEMS, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkRemove(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		stepN     = MaxStackSize / (2 * countPoints)
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDeep := stepDeep - 1
		for range countPoints {
			deep := currDeep
			testCases = append(testCases, testCase{
				name: fmt.Sprintf("%s %s %d %d", opcode.REMOVE, typ, deep, 1),
				f: func() *VM {
					var item stackitem.Item
					if typ == stackitem.ArrayT {
						item = stackitem.NewArray(getArrayWithDeep(deep))
					} else {
						item = stackitem.NewMapWithValue([]stackitem.MapElement{
							{
								Key:   stackitem.NewBigInteger(big.NewInt(0)),
								Value: stackitem.NewArray(getArrayWithDeep(deep - 2)),
							},
						})
					}
					v := opVM(opcode.REMOVE)()
					v.estack.PushVal(item)
					v.estack.PushVal(0)
					return v
				},
			})
			currDeep += stepDeep
		}
		if typ == stackitem.MapT {
			currN := stepN - 1
			for range countPoints {
				n := currN
				testCases = append(testCases, testCase{
					name: fmt.Sprintf("%s %s %d %d", opcode.REMOVE, typ, 1, currN),
					f: func() *VM {
						keys := getUniqueKeys(n, stackitem.MaxKeySize)
						m := make([]stackitem.MapElement, 0, n)
						for _, key := range keys {
							m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
						}
						m[n-1].Value = stackitem.Null{}
						v := opVM(opcode.REMOVE)()
						v.estack.PushVal(stackitem.NewMapWithValue(m))
						v.estack.PushVal(keys[n-1])
						return v
					},
				})
				currN += stepN
			}
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkClearItems(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		stepN     = MaxStackSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDeep := stepDeep - 1
		for range countPoints {
			deep := currDeep
			testCases = append(testCases, testCase{
				name: fmt.Sprintf("%s %s %d %d", opcode.CLEARITEMS, typ, deep, 1),
				f: func() *VM {
					var item stackitem.Item
					if typ == stackitem.ArrayT {
						item = stackitem.NewArray(getArrayWithDeep(deep))
					} else {
						item = stackitem.NewMapWithValue([]stackitem.MapElement{
							{
								Key:   stackitem.NewBigInteger(big.NewInt(0)),
								Value: stackitem.NewArray(getArrayWithDeep(deep - 2)),
							},
						})
					}
					v := opVM(opcode.CLEARITEMS)()
					v.estack.PushVal(item)
					return v
				},
			})
			currDeep += stepDeep
		}
		currN := stepN - 1
		for range countPoints {
			n := currN
			testCases = append(testCases, testCase{
				name: fmt.Sprintf("%s %s %d %d", opcode.CLEARITEMS, typ, 1, n),
				f: func() *VM {
					var item stackitem.Item
					if typ == stackitem.ArrayT {
						item = stackitem.NewArray(make([]stackitem.Item, n))
					} else {
						m := make([]stackitem.MapElement, 0, n/2)
						for i := range n / 2 {
							m = append(m, stackitem.MapElement{
								Key:   stackitem.NewBigInteger(big.NewInt(int64(i))),
								Value: stackitem.Null{},
							})
						}
						item = stackitem.NewMapWithValue(m)
					}
					v := opVM(opcode.CLEARITEMS)()
					v.estack.PushVal(item)
					return v
				},
			})
			currN += stepN
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkPopItem(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		currDeep  = stepDeep - 1
	)
	for range countPoints {
		deep := currDeep
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("%s %s %d", opcode.POPITEM, stackitem.ArrayT, deep),
			f: func() *VM {
				v := opVM(opcode.POPITEM)()
				v.estack.PushVal(stackitem.NewArray(getArrayWithDeep(deep)))
				return v
			},
		})
		currDeep += stepDeep
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkSize(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	testCases := []testCase{
		{
			name:  fmt.Sprintf("%s %s", opcode.SIZE, stackitem.ArrayT),
			items: []any{[]any{}},
		},
		{
			name:  fmt.Sprintf("%s %s", opcode.SIZE, stackitem.MapT),
			items: []any{stackitem.NewMap()},
		},
		{
			name:  fmt.Sprintf("%s %s", opcode.SIZE, stackitem.ByteArrayT),
			items: []any{[]byte{}},
		},
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.SIZE, nil, tc.items...))
		})
	}
}

func BenchmarkJumps(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
		items  []any
	}
	testCases := []testCase{
		{
			name:   fmt.Sprintf("%s", opcode.JMP),
			op:     opcode.JMP,
			params: []byte{0},
		},
		{
			name:   fmt.Sprintf("%s", opcode.JMPIF),
			op:     opcode.JMPIF,
			params: []byte{0},
			items:  []any{true},
		},
		{
			name:   fmt.Sprintf("%s", opcode.JMPEQ),
			op:     opcode.JMPEQ,
			params: []byte{0},
			items:  []any{0, 0},
		},
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, tc.params, tc.items...))
		})
	}
}

func BenchmarkCall(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	testCases := []testCase{
		{
			name: fmt.Sprintf("%s", opcode.CALL),
			f: func() *VM {
				return opParamVM(opcode.CALL, []byte{0})()
			},
		},
		{
			name: fmt.Sprintf("%s", opcode.CALLA),
			f: func() *VM {
				v := opVM(opcode.CALLA)()
				p := stackitem.NewPointerWithHash(0, []byte{42}, v.Context().ScriptHash())
				v.estack.PushVal(p)
				return v
			},
		},
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkKeys(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases  []testCase
		stepMapLen = MaxStackSize / (2 * countPoints)
	)
	for _, typ := range []stackitem.Type{stackitem.IntegerT, stackitem.ByteArrayT} {
		currMapLen := stepMapLen - 1
		for range countPoints {
			m := make([]stackitem.MapElement, 0, currMapLen)
			if typ == stackitem.IntegerT {
				for i := range currMapLen {
					m = append(m, stackitem.MapElement{Key: stackitem.NewBigInteger(big.NewInt(int64(i)))})
				}
			} else {
				keys := getUniqueKeys(currMapLen, stackitem.MaxKeySize)
				for _, key := range keys {
					m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
				}
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d", opcode.KEYS, typ, currMapLen),
				items: []any{stackitem.NewMapWithValue(m)},
			})
			currMapLen += stepMapLen
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.KEYS, nil, tc.items...))
		})
	}
}

func BenchmarkValues(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		stepN     = MaxStackSize / countPoints
	)
	for _, itemTyp := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		for _, clonedTyp := range []stackitem.Type{stackitem.AnyT, stackitem.StructT} {
			currDeep := stepDeep - 2
			for range countPoints {
				var item, clonedItem stackitem.Item
				if clonedTyp == stackitem.StructT {
					clonedItem = stackitem.NewStruct(getStructWithDeep(currDeep))
				} else {
					clonedItem = stackitem.NewArray(getArrayWithDeep(currDeep))
				}
				if itemTyp == stackitem.ArrayT {
					item = stackitem.NewArray([]stackitem.Item{clonedItem})
				} else {
					item = stackitem.NewMapWithValue([]stackitem.MapElement{
						{
							Key:   stackitem.NewBool(true),
							Value: clonedItem,
						},
					})
				}
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %s %d %d", opcode.VALUES, itemTyp, clonedTyp, currDeep, 1),
					items: []any{item},
				})
				currDeep += stepDeep
			}
		}
		currN := stepN - 1
		for range countPoints {
			var (
				item stackitem.Item
				n    int
			)
			if itemTyp == stackitem.ArrayT {
				n = currN
				arr := make([]stackitem.Item, 0, currN)
				for range currN {
					arr = append(arr, stackitem.Null{})
				}
				item = stackitem.NewArray(arr)
			} else {
				n = currN / 2
				m := make([]stackitem.MapElement, 0, currN/2)
				for i := range currN / 2 {
					m = append(m, stackitem.MapElement{
						Key:   stackitem.NewBigInteger(big.NewInt(int64(i))),
						Value: stackitem.Null{},
					})
				}
				item = stackitem.NewMapWithValue(m)
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %s %d %d", opcode.VALUES, itemTyp, stackitem.AnyT, n, n),
				items: []any{item},
			})
			currN += stepN
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.VALUES, nil, tc.items...))
		})
	}
}

func BenchmarkRet(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepDeep  = MaxStackSize / countPoints
		stepN     = MaxStackSize / countPoints
		currDeep  = stepDeep
		currN     = stepN
	)
	for range countPoints {
		deep := currDeep
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("%s %d %d", opcode.RET, deep, 1),
			f: func() *VM {
				v := opVM(opcode.RET)()
				callerCtx := &Context{retCount: 1, sc: &scriptContext{estack: NewStack("call")}}
				callerCtx.sc.estack.PushVal(stackitem.NewArray(getArrayWithDeep(deep - 1)))
				v.istack = append(v.istack, callerCtx)
				v.estack = callerCtx.Estack()
				return v
			},
		})
		currDeep += stepDeep
	}
	for range countPoints {
		n := currN
		testCases = append(testCases, testCase{
			name: fmt.Sprintf("%s %d %d", opcode.RET, n, n),
			f: func() *VM {
				v := opVM(opcode.RET)()
				callerCtx := &Context{retCount: n, sc: &scriptContext{estack: NewStack("call")}}
				for range n {
					callerCtx.sc.estack.PushVal(stackitem.Null{})
				}
				v.istack = append(v.istack, callerCtx)
				v.estack = callerCtx.Estack()
				return v
			},
		})
		currN += stepN
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkHasKey(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepN     = MaxStackSize / (2 * countPoints)
	)
	for _, typ := range []stackitem.Type{stackitem.IntegerT, stackitem.ByteArrayT} {
		currN := stepN - 1
		for range countPoints {
			m := make([]stackitem.MapElement, 0, currN)
			if typ == stackitem.IntegerT {
				for i := range currN {
					m = append(m, stackitem.MapElement{Key: stackitem.Make(i)})
				}
			} else {
				keys := getUniqueKeys(currN, stackitem.MaxKeySize)
				for _, key := range keys {
					m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
				}
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %s %d %s", opcode.HASKEY, stackitem.MapT, currN, typ),
				items: []any{stackitem.NewMapWithValue(m), m[currN-1].Key},
			})
			currN += stepN
		}
	}
	testCases = append(testCases, testCase{
		name:  fmt.Sprintf("%s %s", opcode.HASKEY, stackitem.ArrayT),
		items: []any{[]any{}, 0},
	})
	testCases = append(testCases, testCase{
		name:  fmt.Sprintf("%s %s", opcode.HASKEY, stackitem.ByteArrayT),
		items: []any{[]byte{}, 0},
	})
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.HASKEY, nil, tc.items...))
		})
	}
}

func BenchmarkExceptions(b *testing.B) {
	type testCase struct {
		name  string
		op    opcode.Opcode
		items []any
	}
	testCases := []testCase{
		{
			name:  fmt.Sprintf("%s", opcode.THROW),
			op:    opcode.THROW,
			items: []any{"error"},
		},
		{
			name: fmt.Sprintf("%s", opcode.ABORT),
			op:   opcode.ABORT,
		},
		{
			name:  fmt.Sprintf("%s", opcode.ABORTMSG),
			op:    opcode.ABORTMSG,
			items: []any{"error"},
		},
		{
			name:  fmt.Sprintf("%s", opcode.ASSERT),
			op:    opcode.ASSERT,
			items: []any{false},
		},
		{
			name:  fmt.Sprintf("%s", opcode.ASSERTMSG),
			op:    opcode.ASSERTMSG,
			items: []any{false, "error"},
		},
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, exceptParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkTry(b *testing.B) {
	type testCase struct {
		name   string
		f      func() *VM
		op     opcode.Opcode
		params []byte
		items  []any
	}
	testCases := []testCase{
		{
			name: fmt.Sprintf("%s", opcode.TRY),
			f: func() *VM {
				return opParamVM(opcode.TRY, []byte{1, 0})()
			},
		},
		{
			name: fmt.Sprintf("%s", opcode.ENDTRY),
			f: func() *VM {
				v := opParamVM(opcode.ENDTRY, []byte{0})()
				v.Context().tryStack.PushVal(newExceptionHandlingContext(0, -1))
				return v
			},
		},
		{
			name: fmt.Sprintf("%s", opcode.ENDFINALLY),
			f: func() *VM {
				v := opVM(opcode.ENDFINALLY)()
				v.Context().tryStack.PushVal(&exceptionHandlingContext{EndOffset: 0})
				return v
			},
		},
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}

func BenchmarkOpcodesWithoutAnything(b *testing.B) {
	opcodes := []opcode.Opcode{
		opcode.PUSHM1,
		opcode.PUSH0,
		opcode.PUSHT,
		opcode.PUSHNULL,
		opcode.NEWMAP,
		opcode.NOP,
	}
	for _, op := range opcodes {
		b.Run(fmt.Sprintf("%s", op), func(b *testing.B) {
			benchOpcode(b, opVM(op))
		})
	}
}
