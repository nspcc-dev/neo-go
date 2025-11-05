package vm

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
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

func getArrayWithSizeAndDeep(n, d int) []stackitem.Item {
	items := make([]stackitem.Item, 0, n)
	for range n {
		var item stackitem.Item = stackitem.Null{}
		for range d - 1 {
			item = stackitem.NewArray([]stackitem.Item{item})
		}
		items = append(items, item)
	}
	return items
}

func getStructWithSizeAndDeep(n, d int) []stackitem.Item {
	items := make([]stackitem.Item, 0, n)
	for range n {
		var item stackitem.Item = stackitem.Null{}
		for range d - 1 {
			item = stackitem.NewStruct([]stackitem.Item{item})
		}
		items = append(items, item)
	}
	return items
}

func genUniqueKeys(num, keySize int) [][]byte {
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
		currArrayLen     = stepArrayLen
		currByteArrayLen = stepByteArrayLen - 1
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
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				paramLen := params.Len
				paramDeep := params.Deep
				testCases = append(testCases, testCase{
					name: fmt.Sprintf("%s %d %d", opcode.LDSFLD0, paramLen, paramDeep),
					f: func() *VM {
						v := opVM(opcode.LDSFLD0)()
						slot := []stackitem.Item{stackitem.NewArray(getArrayWithSizeAndDeep(paramLen, paramDeep))}
						v.Context().sc.static = slot
						return v
					},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
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
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ removedLen, removedDeep, addedLen, addedDeep int }{
				{currIndependentParam, currDependentParam, 1, 1},
				{currDependentParam, currIndependentParam, 1, 1},
				{1, 1, currIndependentParam, currDependentParam},
				{1, 1, currDependentParam, currIndependentParam},
			} {
				slot := []stackitem.Item{stackitem.NewArray(getArrayWithSizeAndDeep(params.removedLen, params.removedDeep))}
				item := stackitem.NewArray(getArrayWithSizeAndDeep(params.addedLen, params.addedDeep))
				testCases = append(testCases, testCase{
					name: fmt.Sprintf("%s %d %d %d %d", opcode.STSFLD0, params.removedLen, params.removedDeep, params.addedLen, params.addedDeep),
					f: func() *VM {
						v := opParamPushVM(opcode.STSFLD0, nil, item)()
						v.Context().sc.static = slot
						v.refs.Add(slot[0])
						return v
					},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
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
		name  string
		items []any
	}
	var (
		testCases        []testCase
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		for _, op := range []opcode.Opcode{opcode.SUBSTR, opcode.LEFT, opcode.RIGHT} {
			items := []any{make([]byte, currByteArrayLen)}
			if op == opcode.SUBSTR {
				items = append(items, 0)
			}
			items = append(items, currByteArrayLen)
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, currByteArrayLen),
				items: items,
			})
		}
		currByteArrayLen += stepByteArrayLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.CAT, nil, tc.items...))
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
		case opcode.DROP:
			items = []any{arr}
		case opcode.NIP, opcode.XDROP:
			items = []any{nil, arr}
		case opcode.CLEAR:
			for _, item := range arr {
				items = append(items, item)
			}
		}
		return items
	}
	var (
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				arr := getArrayWithSizeAndDeep(params.Len, params.Deep)
				for _, op := range []opcode.Opcode{opcode.DROP, opcode.NIP, opcode.XDROP, opcode.CLEAR} {
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %d %d", op, params.Len, params.Deep),
						op:    op,
						items: itemsForOpcode(op, arr),
					})
				}
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
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
		testCases            []testCase
		stepStackSize        = MaxStackSize / countPoints
		stepByteArrayLen     = stackitem.MaxSize / countPoints
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currStackSize        = stepStackSize
		currByteArrayLen     = stepByteArrayLen
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		items := make([]any, currStackSize)
		items = append(items, currStackSize-1)
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d", opcode.PICK, stackitem.AnyT, currStackSize),
			op:    opcode.PICK,
			items: items,
		})
		currStackSize += stepStackSize
	}
	for range countPoints {
		item := make([]byte, currByteArrayLen)
		for _, op := range []opcode.Opcode{opcode.DUP, opcode.OVER, opcode.TUCK, opcode.PICK} {
			name := fmt.Sprintf("%s %s %d", op, stackitem.ByteArrayT, currByteArrayLen)
			if op == opcode.PICK {
				name += " 1"
			}
			testCases = append(testCases, testCase{
				name:  name,
				op:    op,
				items: itemsForOpcode(op, item),
			})
		}
		currByteArrayLen += stepByteArrayLen
	}
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				item := getArrayWithSizeAndDeep(params.Len, params.Deep)
				for _, op := range []opcode.Opcode{opcode.DUP, opcode.OVER, opcode.TUCK, opcode.PICK} {
					name := fmt.Sprintf("%s %s %d %d", op, stackitem.ArrayT, params.Len, params.Deep)
					if op == opcode.PICK {
						name += " 1"
					}
					testCases = append(testCases, testCase{
						name:  name,
						op:    op,
						items: itemsForOpcode(op, item),
					})
				}
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
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
			items := make([]any, currStackSize)
			items = append(items, currStackSize-1)
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, currStackSize),
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
		opcode.MUL,
		opcode.DIV,
		opcode.MOD,
		opcode.POW,
		opcode.NUMEQUAL,
		opcode.NUMNOTEQUAL,
		opcode.LT,
		opcode.LE,
		opcode.GT,
		opcode.GE,
		opcode.MIN,
		opcode.MAX,
		opcode.SHL,
		opcode.SHR,
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
		testCases  []testCase
		stepExp    = math.MaxUint / countPoints
		stepSHLArg = maxSHLArg / countPoints
		bigInts1   = getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
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
			if op == opcode.POW {
				currExp := stepExp
				for range countPoints {
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %d %d", op, bigInts1[i].BitLen(), currExp),
						op:    op,
						items: []any{bigInts1[i], big.NewInt(int64(currExp))},
					})
					currExp += stepExp
				}
				continue
			}
			if op == opcode.SHL || op == opcode.SHR {
				currSHLArg := stepSHLArg
				for range countPoints {
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %d %d", op, bigInts1[i].BitLen(), currSHLArg),
						op:    op,
						items: []any{bigInts1[i], big.NewInt(int64(currSHLArg))},
					})
					currSHLArg += stepExp
				}
			}
			testCases = append(testCases, testCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i], bigInts2[i]},
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
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", opcode.APPEND, stackitem.ArrayT, params.Len, params.Deep),
					items: []any{[]any{}, getStructWithSizeAndDeep(params.Len, params.Deep)},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.APPEND, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkPackMap(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = (MaxStackSize - currIndependentParam) / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				values := getArrayWithSizeAndDeep(params.Len, params.Deep)
				keys := genUniqueKeys(params.Len, stackitem.MaxKeySize)
				items := make([]any, 0, 2*params.Len+1)
				for i := range params.Len {
					items = append(items, values[i], keys[i])
				}
				items = append(items, params.Len)
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %d %d", opcode.PACKMAP, params.Len, params.Deep),
					items: items,
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
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
		op    opcode.Opcode
		items []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				items := make([]any, 0, params.Len)
				for _, item := range getArrayWithSizeAndDeep(params.Len, params.Deep) {
					items = append(items, item)
				}
				items = append(items, params.Len)
				for _, op := range []opcode.Opcode{opcode.PACKSTRUCT, opcode.PACK} {
					testCases = append(testCases, testCase{
						name:  fmt.Sprintf("%s %d %d", op, params.Len, params.Deep),
						op:    op,
						items: items,
					})
				}
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, nil, tc.items...))
		})
	}
}

func BenchmarkUnpack(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				items := []any{getArrayWithSizeAndDeep(params.Len, params.Deep)}
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %d %d", opcode.UNPACK, params.Len, params.Deep),
					items: items,
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
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
		testCases            []testCase
		stepMapSize          = MaxStackSize / (2 * countPoints)
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currMapSize          = stepMapSize
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				item := stackitem.NewArray(getArrayWithSizeAndDeep(params.Len, params.Deep))
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", opcode.PICKITEM, stackitem.ArrayT, params.Len, params.Deep),
					items: []any{[]any{item}, 0},
				})
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d %d", opcode.PICKITEM, stackitem.MapT, params.Len, params.Deep, 1),
					items: []any{stackitem.NewMapWithValue([]stackitem.MapElement{{Key: stackitem.Make(0), Value: item}}), 0},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for range countPoints {
		m := make([]stackitem.MapElement, 0, currMapSize)
		keys := genUniqueKeys(currMapSize, stackitem.MaxKeySize)
		for _, key := range keys {
			m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
		}
		m[currMapSize-1].Value = stackitem.Null{}
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d %d %d", opcode.PICKITEM, stackitem.MapT, 1, 1, currMapSize),
			items: []any{stackitem.NewMapWithValue(m), m[currMapSize-1].Key},
		})
		currMapSize += stepMapSize
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.PICKITEM, nil, tc.items...))
		})
	}
}

func BenchmarkSetItem(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepMapSize          = MaxStackSize / countPoints
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currMapSize          = stepMapSize
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ removedLen, removedDeep, addedLen, addedDeep int }{
				{currIndependentParam, currDependentParam, 1, 1},
				{currDependentParam, currIndependentParam, 1, 1},
				{1, 1, currIndependentParam, currDependentParam},
				{1, 1, currDependentParam, currIndependentParam},
			} {
				removedItem := stackitem.NewArray(getArrayWithSizeAndDeep(params.removedLen, params.removedDeep))
				addedItem := stackitem.NewArray(getArrayWithSizeAndDeep(params.addedLen, params.addedDeep))
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d %d %d %d", opcode.SETITEM, stackitem.MapT, params.removedLen, params.removedDeep, params.addedLen, params.addedDeep, 1),
					items: []any{stackitem.NewMapWithValue([]stackitem.MapElement{{Key: stackitem.Make(0), Value: removedItem}}), 0, addedItem},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for range countPoints {
		m := make([]stackitem.MapElement, 0, currMapSize)
		keys := genUniqueKeys(currMapSize, stackitem.MaxKeySize)
		for _, key := range keys {
			m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
		}
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d %d %d %d %d", opcode.SETITEM, stackitem.MapT, 1, 1, 1, 1, currMapSize),
			items: []any{stackitem.NewMapWithValue(m), m[currMapSize-1].Key, nil},
		})
		currMapSize += stepMapSize
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.SETITEM, nil, 0, 0, 0, false, tc.items...))
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
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepMapSize          = MaxStackSize / countPoints
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currMapSize          = stepMapSize
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				item := stackitem.NewArray(getArrayWithSizeAndDeep(params.Len, params.Deep))
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", opcode.REMOVE, stackitem.ArrayT, params.Len, params.Deep),
					items: []any{[]any{item}, 0},
				})
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d %d", opcode.REMOVE, stackitem.MapT, params.Len, params.Deep, 1),
					items: []any{stackitem.NewMapWithValue([]stackitem.MapElement{{Key: stackitem.Make(0), Value: item}}), 0},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for range countPoints {
		m := make([]stackitem.MapElement, 0, currMapSize)
		keys := genUniqueKeys(currMapSize, stackitem.MaxKeySize)
		for _, key := range keys {
			m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
		}
		m[currMapSize-1].Value = stackitem.Null{}
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d %d %d", opcode.REMOVE, stackitem.MapT, 1, 1, currMapSize),
			items: []any{stackitem.NewMapWithValue(m), m[currMapSize-1].Key},
		})
		currMapSize += stepMapSize
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamSlotsPushReadOnlyVM(opcode.REMOVE, nil, 0, 0, 0, false, tc.items...))
		})
	}
}

func BenchmarkClearItems(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", opcode.CLEARITEMS, stackitem.ArrayT, params.Len, params.Deep),
					items: []any{getArrayWithSizeAndDeep(params.Len, params.Deep)},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.CLEARITEMS, nil, tc.items...))
		})
	}
}

func BenchmarkPopItem(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", opcode.POPITEM, stackitem.ArrayT, params.Len, params.Deep),
					items: []any{[]any{getArrayWithSizeAndDeep(params.Len, params.Deep)}},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.POPITEM, nil, tc.items...))
		})
	}
}

func BenchmarkSize(b *testing.B) {
	type testCase struct {
		name  string
		items []any
	}
	var (
		testCases []testCase
		stepLen   = MaxStackSize / countPoints
		currLen   = stepLen
	)
	for range countPoints {
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d", opcode.SIZE, stackitem.ArrayT, currLen),
			items: []any{make([]any, currLen)},
		})
		currLen += stepLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.SIZE, nil, tc.items...))
		})
	}
}

func BenchmarkCall(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepLen   = transaction.MaxScriptLength / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.CALL, opcode.CALLL, opcode.CALLA} {
		currLen := stepLen
		for range countPoints {
			var (
				items  []any
				params []byte
				pos    = currLen - 1
			)
			if op == opcode.CALLA {
				items = []any{stackitem.NewPointer(pos, make([]byte, currLen))}
			} else {
				params = make([]byte, 4)
				binary.LittleEndian.PutUint32(params, uint32(pos))
			}
			testCases = append(testCases, testCase{
				name: fmt.Sprintf("%s %d", op, pos),
				f: func() *VM {
					v := opParamPushVM(op, params, items...)()
					v.Context().sc.prog = make([]byte, pos+1)
					return v
				},
			})
			currLen += stepLen
		}
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
		testCases []testCase
		stepLen   = MaxStackSize / countPoints
		currLen   = stepLen
		key       = make([]byte, stackitem.MaxKeySize)
	)
	for range countPoints {
		m := make([]stackitem.MapElement, 0, currLen)
		for range currLen {
			m = append(m, stackitem.MapElement{
				Key: stackitem.NewByteArray(key),
			})
		}
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %d", opcode.KEYS, currLen),
			items: []any{stackitem.NewMapWithValue(m)},
		})
		currLen += stepLen
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
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				testCases = append(testCases, testCase{
					name:  fmt.Sprintf("%s %s %d %d", opcode.VALUES, stackitem.StructT, params.Len, params.Deep),
					items: []any{stackitem.NewStruct(getStructWithSizeAndDeep(params.Len, params.Deep))},
				})
			}
			currDependentParam += stepDependentParam
		}
		currIndependentParam += stepIndependentParam
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.VALUES, nil, tc.items...))
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
		stepLen   = MaxStackSize / countPoints
		currLen   = stepLen
		key       = make([]byte, stackitem.MaxKeySize)
	)
	key[stackitem.MaxKeySize-1] = 1
	for range countPoints {
		m := make([]stackitem.MapElement, 0, currLen)
		for range currLen {
			m = append(m, stackitem.MapElement{
				Key: stackitem.NewByteArray(key),
			})
		}
		testCases = append(testCases, testCase{
			name:  fmt.Sprintf("%s %s %d", opcode.HASKEY, stackitem.MapT, currLen),
			items: []any{stackitem.NewMapWithValue(m), make([]byte, stackitem.MaxKeySize)},
		})
		currLen += stepLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(opcode.HASKEY, nil, tc.items...))
		})
	}
}

func BenchmarkOpcodesWithoutAnything(b *testing.B) {
	opcodes := []opcode.Opcode{
		opcode.PUSHM1,
		opcode.PUSH0,
		opcode.PUSH1,
		opcode.PUSH2,
		opcode.PUSH3,
		opcode.PUSH4,
		opcode.PUSH5,
		opcode.PUSH6,
		opcode.PUSH7,
		opcode.PUSH8,
		opcode.PUSH9,
		opcode.PUSH10,
		opcode.PUSH11,
		opcode.PUSH12,
		opcode.PUSH13,
		opcode.PUSH14,
		opcode.PUSH15,
		opcode.PUSH16,
		opcode.PUSHT,
		opcode.PUSHF,
		opcode.PUSHNULL,
		opcode.NEWMAP,
	}
	for _, op := range opcodes {
		b.Run(fmt.Sprintf("%s", op), func(b *testing.B) {
			benchOpcode(b, opVM(op))
		})
	}
}

func BenchmarkOthersOpcodes(b *testing.B) {
	type testCase struct {
		name   string
		op     opcode.Opcode
		params []byte
		items  []any
	}
	var (
		testCases            []testCase
		stepIndependentParam = int(math.Sqrt(float64(MaxStackSize))) / countPoints
		currIndependentParam = stepIndependentParam
	)
	for range countPoints {
		var (
			stepDependentParam = MaxStackSize / (countPoints * currIndependentParam)
			currDependentParam = stepDependentParam
		)
		for range countPoints {
			for _, params := range []struct{ Len, Deep int }{{currIndependentParam, currDependentParam}, {currDependentParam, currIndependentParam}} {
				items := []any{stackitem.NewArray(getArrayWithSizeAndDeep(params.Len, params.Deep))}
				for _, op := range []opcode.Opcode{opcode.ISNULL, opcode.ISTYPE, opcode.THROW} {
					testCases = append(testCases, testCase{
						name:   fmt.Sprintf("%s %d %d", op, params.Len, params.Deep),
						op:     op,
						params: []byte{0},
						items:  items,
					})
				}
			}
			currDependentParam += stepDependentParam
		}
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(tc.op, tc.params, tc.items...))
		})
	}
}

func BenchmarkJumps(b *testing.B) {
	type testCase struct {
		name string
		f    func() *VM
	}
	var (
		testCases []testCase
		stepLen   = transaction.MaxScriptLength / countPoints
		currLen   = stepLen
	)
	for range countPoints {
		var (
			params = make([]byte, 4)
			pos    = currLen - 1
		)
		binary.LittleEndian.PutUint32(params, uint32(pos))
		for _, op := range []opcode.Opcode{opcode.JMP, opcode.JMPIF, opcode.JMPEQ} {
			var items []any
			if op == opcode.JMPIF {
				items = []any{true}
			} else if op == opcode.JMPEQ {
				items = []any{0, 0}
			}
			testCases = append(testCases, testCase{
				name: fmt.Sprintf("%s %d", op, pos),
				f: func() *VM {
					v := opParamPushVM(op, params, items...)()
					v.Context().sc.prog = make([]byte, currLen)
					return v
				},
			})
		}
		currLen += stepLen
	}
	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchOpcode(b, tc.f)
		})
	}
}
