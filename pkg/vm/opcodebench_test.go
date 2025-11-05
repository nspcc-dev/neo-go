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

type benchCase struct {
	name   string
	op     opcode.Opcode
	params []byte
	items  []any
	f      func() *VM
}

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

func opParamPushVM(bc benchCase) func() *VM {
	return func() *VM {
		script := []byte{byte(bc.op)}
		script = append(script, bc.params...)
		v := load(script)
		v.SyscallHandler = func(_ *VM, _ uint32) error {
			return nil
		}
		for i := range bc.items {
			item, ok := bc.items[i].(stackitem.Item)
			if ok {
				item = stackitem.DeepCopy(item, true)
			} else {
				item = stackitem.Make(bc.items[i])
			}
			v.estack.PushVal(item)
		}
		return v
	}
}

func exceptParamPushVM(op opcode.Opcode, param []byte, items ...any) func() *VM {
	return func() *VM {
		v := opParamPushVM(benchCase{op: op, params: param, items: items})()
		v.Context().tryStack.PushVal(newExceptionHandlingContext(0, 0))
		return v
	}
}

const countPoints = 8

func getBigInts(minBits, maxBits, num uint) []*big.Int {
	var (
		step = (maxBits - minBits) / num
		res  = make([]*big.Int, num)
	)
	res[0] = new(big.Int).Lsh(big.NewInt(1), minBits+step-1)
	for i := range num {
		res[i] = new(big.Int).Lsh(res[i-1], step)
	}
	return res
}

func getArrayWithDepth(d int) []stackitem.Item {
	var item stackitem.Item = stackitem.Null{}
	for range d - 1 {
		item = stackitem.NewArray([]stackitem.Item{item})
	}
	return []stackitem.Item{item}
}

func getStructWithDepth(d int) []stackitem.Item {
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

func Benchmark_IS(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 2*countPoints)
		stepDepth  = MaxStackSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.ISNULL, opcode.ISTYPE} {
		currDepth := stepDepth - 1
		for range countPoints {
			benchCases = append(benchCases, benchCase{
				name:   fmt.Sprintf("%s %d", op, currDepth),
				op:     op,
				params: []byte{0},
				items:  []any{stackitem.NewArray(getArrayWithDepth(currDepth))},
			})
			currDepth += stepDepth
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_PUSHINT(b *testing.B) {
	var (
		benchCases   = make([]benchCase, 0, 6)
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
		benchCases = append(benchCases, benchCase{
			name:   fmt.Sprintf("%s %d", op, numBytes),
			op:     op,
			params: buf,
		})
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_PUSHDATA(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, countPoints)
		stepLen    = stackitem.MaxSize / countPoints
		currLen    = stepLen
	)
	for range countPoints {
		benchCases = append(benchCases, benchCase{
			name:   fmt.Sprintf("%s %d", opcode.PUSHDATA4, currLen),
			params: make([]byte, currLen),
		})
		currLen += stepLen
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_CONVERT(b *testing.B) {
	var (
		benchCases       = make([]benchCase, 0, 2*countPoints)
		stepArrayLen     = MaxStackSize / countPoints
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currArrayLen     = stepArrayLen - 1
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		benchCases = append(benchCases, benchCase{
			name:   fmt.Sprintf("%s %s %s %d", opcode.CONVERT, stackitem.ArrayT, stackitem.StructT, currArrayLen),
			params: []byte{byte(stackitem.StructT)},
			items:  []any{make([]any, currArrayLen)},
		})
		currArrayLen += stepArrayLen
	}
	for range countPoints {
		benchCases = append(benchCases, benchCase{
			name:   fmt.Sprintf("%s %s %s %d", opcode.CONVERT, stackitem.ByteArrayT, stackitem.BufferT, currByteArrayLen),
			params: []byte{byte(stackitem.BufferT)},
			items:  []any{stackitem.NewByteArray(make([]byte, currByteArrayLen))},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_INITSLOT(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 3*countPoints)
		stepSlots  = byte(math.MaxUint8 / countPoints)
		currSlots  = stepSlots
	)
	for range countPoints {
		for _, params := range []struct{ localSlots, argSlots byte }{
			{currSlots, 0},
			{0, currSlots},
		} {
			benchCases = append(benchCases, benchCase{
				name:   fmt.Sprintf("%s %d %d", opcode.INITSLOT, params.localSlots, params.argSlots),
				op:     opcode.INITSLOT,
				params: []byte{params.localSlots, params.argSlots},
				items:  make([]any, params.argSlots),
			})
		}
		benchCases = append(benchCases, benchCase{
			name:   fmt.Sprintf("%s %d", opcode.INITSSLOT, currSlots),
			op:     opcode.INITSSLOT,
			params: []byte{currSlots},
		})
		currSlots += stepSlots
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_LDSFLD(b *testing.B) {
	b.Run(opcode.LDSFLD.String(), func(b *testing.B) {
		benchOpcode(b, func() *VM {
			v := opParamPushVM(benchCase{op: opcode.LDSFLD, params: []byte{0}})()
			v.Context().sc.static = []stackitem.Item{stackitem.Null{}}
			return v
		})
	})
}

func Benchmark_STSFLD(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 2*countPoints)
		stepDepth  = MaxStackSize / countPoints
		currDepth  = stepDepth
	)
	for range countPoints {
		for _, params := range []struct{ removedDepth, addedDepth int }{
			{currDepth, 1},
			{1, currDepth},
		} {
			removedDepth := params.removedDepth
			addedDepth := params.addedDepth
			benchCases = append(benchCases, benchCase{
				name: fmt.Sprintf("%s %d %d", opcode.STSFLD0, removedDepth, addedDepth),
				f: func() *VM {
					item := stackitem.NewArray(getArrayWithDepth(addedDepth - 1))
					v := opParamPushVM(benchCase{op: opcode.STSFLD0, items: []any{item}})()
					slot := []stackitem.Item{stackitem.NewArray(getArrayWithDepth(removedDepth - 1))}
					v.Context().sc.static = slot
					v.refs.Add(slot[0])
					return v
				},
			})
		}
		currDepth += stepDepth
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_NEW(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 5*countPoints)
		//stepArrayLen     = MaxStackSize / countPoints
		stepByteArrayLen = stackitem.MaxSize / countPoints
		//currArrayLen     = stepArrayLen - 1
		currByteArrayLen = stepByteArrayLen
	)
	/*for range countPoints {
		for _, typ := range []stackitem.Type{stackitem.BooleanT, stackitem.IntegerT, stackitem.ByteArrayT, stackitem.AnyT} {
			benchCases = append(benchCases, benchCase{
				name:   fmt.Sprintf("%s %s %d", opcode.NEWARRAYT, typ, currArrayLen),
				op:     opcode.NEWARRAYT,
				params: []byte{byte(typ)},
				items:  []any{currArrayLen},
			})
		}
		benchCases = append(benchCases, benchCase{
			name:  fmt.Sprintf("%s %d", opcode.NEWARRAY, currArrayLen),
			op:    opcode.NEWARRAY,
			items: []any{currArrayLen},
		})
		currArrayLen += stepArrayLen
	}*/
	for range countPoints {
		benchCases = append(benchCases, benchCase{
			name:  fmt.Sprintf("%s %d", opcode.NEWBUFFER, currByteArrayLen),
			op:    opcode.NEWBUFFER,
			items: []any{currByteArrayLen},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_MEMCPY(b *testing.B) {
	var (
		benchCases       = make([]benchCase, 0, countPoints)
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		buf1 := stackitem.NewBuffer(make([]byte, currByteArrayLen))
		buf2 := stackitem.NewBuffer(make([]byte, currByteArrayLen))
		benchCases = append(benchCases, benchCase{
			name: fmt.Sprintf("%s %d", opcode.MEMCPY, currByteArrayLen),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.MEMCPY})()
				v.estack.PushVal(buf1)
				v.estack.PushVal(0)
				v.estack.PushVal(buf2)
				v.estack.PushVal(0)
				v.estack.PushVal(currByteArrayLen)
				return v
			},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_CAT(b *testing.B) {
	var (
		benchCases       = make([]benchCase, 0, countPoints)
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		benchCases = append(benchCases, benchCase{
			name: fmt.Sprintf("%s %d", opcode.CAT, currByteArrayLen),
			items: []any{
				[]byte{},
				make([]byte, currByteArrayLen),
			},
		})
		currByteArrayLen += stepByteArrayLen
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_SUBSTR_LEFT(b *testing.B) {
	var (
		benchCases       = make([]benchCase, 0, 2*countPoints)
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
			benchCases = append(benchCases, benchCase{
				op:    op,
				name:  fmt.Sprintf("%s %d", op, currByteArrayLen),
				items: items,
			})
			currByteArrayLen += stepByteArrayLen
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_DROP(b *testing.B) {
	/*itemsForOpcode := func(op opcode.Opcode, arr []stackitem.Item) []any {
		var items []any
		switch op {
		case opcode.DROP, opcode.CLEAR:
			items = []any{arr}
		case opcode.NIP, opcode.XDROP:
			items = []any{arr, 0}
		default:
		}
		return items
	}*/
	var (
		benchCases = make([]benchCase, 0, 4*countPoints)
		//stepDepth     = MaxStackSize / countPoints
		stepStackSize = MaxStackSize / countPoints
	)
	/*for _, op := range []opcode.Opcode{opcode.DROP, opcode.NIP, opcode.XDROP, opcode.CLEAR} {
		currDepth := stepDepth
		for range countPoints {
			name := fmt.Sprintf("%s %d", op, currDepth)
			switch op {
			case opcode.XDROP:
				name += " 0"
			case opcode.CLEAR:
				name += " 1"
			default:
			}
			benchCases = append(benchCases, benchCase{
				name:  name,
				op:    op,
				items: itemsForOpcode(op, getArrayWithDepth(currDepth-1)),
			})
			currDepth += stepDepth
		}
	}*/
	for _, op := range []opcode.Opcode{opcode.XDROP /*, opcode.CLEAR*/} {
		currStackSize := stepStackSize
		for range countPoints {
			items := make([]any, currStackSize)
			items[currStackSize-1] = currStackSize - 2
			name := fmt.Sprintf("%s %d %d", op, 1, currStackSize-2)
			if op == opcode.CLEAR {
				name = fmt.Sprintf("%s %d %d", op, currStackSize, currStackSize)
			}
			benchCases = append(benchCases, benchCase{
				name:  name,
				op:    op,
				items: items,
			})
			currStackSize += stepStackSize
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_DUP(b *testing.B) {
	itemsForOpcode := func(op opcode.Opcode, item any) []any {
		var items []any
		switch op {
		case opcode.TUCK:
			items = []any{nil, item}
		case opcode.DUP:
			items = []any{item}
		case opcode.OVER, opcode.PICK:
			items = []any{item, 0}
		default:
		}
		return items
	}
	var (
		benchCases       = make([]benchCase, 0, 4*(countPoints+1))
		stepByteArrayLen = stackitem.MaxSize / countPoints
	)
	for _, op := range []opcode.Opcode{opcode.DUP, opcode.OVER, opcode.TUCK, opcode.PICK} {
		benchCases = append(benchCases, benchCase{
			name:  fmt.Sprintf("%s %s", op, stackitem.AnyT),
			op:    op,
			items: itemsForOpcode(op, nil),
		})
		currByteArrayLen := stepByteArrayLen
		for range countPoints {
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %d", op, stackitem.ByteArrayT, currByteArrayLen),
				op:    op,
				items: itemsForOpcode(op, make([]byte, currByteArrayLen)),
			})
			currByteArrayLen += stepByteArrayLen
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_ROLL_REVERSE(b *testing.B) {
	var (
		benchCases    = make([]benchCase, 0, 2*countPoints)
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
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %d", op, n),
				op:    op,
				items: items,
			})
			currStackSize += stepStackSize
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_INTEGER_OPERANDS(b *testing.B) {
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
	var (
		benchCases  = make([]benchCase, 0, (len(unaryOpcodes)+len(binaryOpcodes)+len(ternaryOpcodes))*countPoints)
		stepNumBits = (stackitem.MaxBigIntegerSizeBits - 1) / countPoints
		bigInts1    = getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
	)
	for _, op := range unaryOpcodes {
		for i := range bigInts1 {
			benchCases = append(benchCases, benchCase{
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
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i], n},
			})
		}
	}
	bigInts3 := getBigInts(0, stackitem.MaxBigIntegerSizeBits-1, countPoints)
	for _, op := range ternaryOpcodes {
		for i := range bigInts1 {
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %d", op, bigInts1[i].BitLen()),
				op:    op,
				items: []any{bigInts1[i], bigInts2[i], bigInts3[i]},
			})
		}
	}
	mulBigInts := getBigInts(0, (stackitem.MaxBigIntegerSizeBits/2)-1, countPoints)
	for _, n := range mulBigInts {
		benchCases = append(benchCases, benchCase{
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
			default:
			}
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %d", op, currNumBits),
				op:    op,
				items: items,
			})
			currNumBits += stepNumBits
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_APPEND(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 2*countPoints)
		stepDepth  = MaxStackSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.StructT} {
		currDepth := stepDepth - 1
		for range countPoints {
			var item stackitem.Item
			if typ == stackitem.ArrayT {
				item = stackitem.NewArray(getArrayWithDepth(currDepth))
			} else {
				item = stackitem.NewStruct(getStructWithDepth(currDepth))
			}
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %s %d", opcode.APPEND, stackitem.ArrayT, typ, currDepth),
				items: []any{[]any{}, item},
			})
			currDepth += stepDepth
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_PACKMAP(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 3*countPoints)
		stepDepth  = MaxStackSize / countPoints
		stepN      = MaxStackSize / (2 * countPoints)
		currDepth  = stepDepth - 1
	)
	for range countPoints {
		benchCases = append(benchCases, benchCase{
			name:  fmt.Sprintf("%s %s %d %d", opcode.PACKMAP, stackitem.AnyT, currDepth, 1),
			items: []any{stackitem.NewArray(getArrayWithDepth(currDepth - 2)), true, 1},
		})
		currDepth += stepDepth
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
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.PACKMAP, typ, 2*currN, currN),
				items: items,
			})
			currN += stepN
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_PACK(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 2*countPoints)
		stepDepth  = MaxStackSize / countPoints
		stepN      = MaxStackSize / countPoints
		currDepth  = stepDepth - 1
		currN      = stepN - 1
	)
	for range countPoints {
		benchCases = append(benchCases, benchCase{
			name:  fmt.Sprintf("%s %d %d", opcode.PACK, currDepth, 1),
			items: []any{stackitem.NewArray(getArrayWithDepth(currDepth - 1)), 1},
		})
		currDepth += stepDepth
	}
	for range countPoints {
		items := make([]any, currN)
		items = append(items, currN)
		benchCases = append(benchCases, benchCase{
			name:  fmt.Sprintf("%s %d %d", opcode.PACK, currN, currN),
			items: items,
		})
		currN += stepN
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_UNPACK(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 4*countPoints)
		stepDepth  = MaxStackSize / countPoints
		stepN      = MaxStackSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDepth := stepDepth - 1
		for range countPoints {
			var item stackitem.Item
			if typ == stackitem.ArrayT {
				item = stackitem.NewArray(getArrayWithDepth(currDepth))
			} else {
				item = stackitem.NewMapWithValue([]stackitem.MapElement{{
					Key:   stackitem.NewBool(true),
					Value: stackitem.NewArray(getArrayWithDepth(currDepth - 2)),
				}})
			}
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.UNPACK, typ, currDepth, 1),
				items: []any{item},
			})
			currDepth += stepDepth
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
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %d %d", opcode.UNPACK, typ, currN, currN),
				items: []any{item},
			})
			currN += stepN
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_PICKITEM(b *testing.B) {
	var (
		benchCases       = make([]benchCase, 0, 5*countPoints)
		stepDepth        = MaxStackSize / countPoints
		stepN            = MaxStackSize / (2 * countPoints)
		stepByteArrayLen = stackitem.MaxSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDepth := stepDepth - 1
		for range countPoints {
			var item stackitem.Item
			if typ == stackitem.ArrayT {
				item = stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray(getArrayWithDepth(currDepth)),
				})
			} else {
				item = stackitem.NewMapWithValue([]stackitem.MapElement{{
					Key:   stackitem.NewBigInteger(big.NewInt(0)),
					Value: stackitem.NewArray(getArrayWithDepth(currDepth)),
				}})
			}
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %d %d %s", opcode.PICKITEM, typ, currDepth, 1, stackitem.ArrayT),
				items: []any{item, 0},
			})
			currDepth += stepDepth
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
				benchCases = append(benchCases, benchCase{
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
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %d %d %s %d", opcode.PICKITEM, typ, 1, 1, stackitem.ByteArrayT, currByteArrayLen),
				items: []any{item, 0},
			})
			currByteArrayLen += stepByteArrayLen
		}
	}
	benchCases = append(benchCases, benchCase{
		name:  fmt.Sprintf("%s %s %d %d %s", opcode.PICKITEM, stackitem.ByteArrayT, 1, 1, stackitem.IntegerT),
		items: []any{[]byte{42}, 0},
	})
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_SETITEM(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 5*countPoints)
		stepDepth  = MaxStackSize / countPoints
		stepN      = MaxStackSize / (2 * countPoints)
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDepth := stepDepth - 1
		for range countPoints {
			for _, params := range []struct{ removedDepth, addedDepth int }{
				{currDepth, 1},
				{1, currDepth},
			} {
				removedDepth := params.removedDepth
				addedDepth := params.addedDepth
				benchCases = append(benchCases, benchCase{
					name: fmt.Sprintf("%s %s %d %d %d", opcode.SETITEM, typ, removedDepth, addedDepth, 1),
					f: func() *VM {
						var item, addedItem stackitem.Item
						if typ == stackitem.ArrayT {
							item = stackitem.NewArray(getArrayWithDepth(removedDepth))
							addedItem = stackitem.NewArray(getArrayWithDepth(addedDepth - 1))
						} else {
							item = stackitem.NewMapWithValue([]stackitem.MapElement{
								{
									Key:   stackitem.NewBigInteger(big.NewInt(0)),
									Value: stackitem.NewArray(getArrayWithDepth(removedDepth - 2)),
								},
							})
							addedItem = stackitem.NewArray(getArrayWithDepth(addedDepth - 2))
						}
						v := opParamPushVM(benchCase{op: opcode.SETITEM})()
						v.estack.PushVal(item)
						v.estack.PushVal(0)
						v.estack.PushVal(addedItem)
						return v
					},
				})
			}
			currDepth += stepDepth
		}
		if typ == stackitem.MapT {
			currN := stepN - 1
			for range countPoints {
				n := currN
				benchCases = append(benchCases, benchCase{
					name: fmt.Sprintf("%s %s %d %d %d", opcode.SETITEM, typ, 1, 1, currN),
					f: func() *VM {
						keys := getUniqueKeys(n, stackitem.MaxKeySize)
						m := make([]stackitem.MapElement, 0, n)
						for _, key := range keys {
							m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
						}
						m[n-1].Value = stackitem.Null{}
						v := opParamPushVM(benchCase{op: opcode.SETITEM})()
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
	benchCases = append(benchCases, benchCase{
		name: fmt.Sprintf("%s %s", opcode.SETITEM, stackitem.BufferT),
		f: func() *VM {
			v := opParamPushVM(benchCase{op: opcode.SETITEM})()
			v.estack.PushVal(stackitem.NewBuffer([]byte{42}))
			v.estack.PushVal(0)
			v.estack.PushVal(42)
			return v
		},
	})
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_REVERSEITEMS(b *testing.B) {
	var (
		benchCases       = make([]benchCase, 0, 2*countPoints)
		stepArrayLen     = MaxStackSize / countPoints
		stepByteArrayLen = stackitem.MaxSize / countPoints
		currArrayLen     = stepArrayLen
		currByteArrayLen = stepByteArrayLen
	)
	for range countPoints {
		benchCases = append(benchCases, benchCase{
			name: fmt.Sprintf("%s %s %d", opcode.REVERSEITEMS, stackitem.ArrayT, currArrayLen),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.REVERSEITEMS})()
				v.estack.PushVal(make([]any, currArrayLen))
				return v
			},
		})
		benchCases = append(benchCases, benchCase{
			name: fmt.Sprintf("%s %s %d", opcode.REVERSEITEMS, stackitem.BufferT, currByteArrayLen),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.REVERSEITEMS})()
				v.estack.PushVal(stackitem.NewBuffer(make([]byte, currByteArrayLen)))
				return v
			},
		})
		currArrayLen += stepArrayLen
		currByteArrayLen += stepByteArrayLen
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_REMOVE(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 3*countPoints)
		stepDepth  = MaxStackSize / countPoints
		stepN      = MaxStackSize / (2 * countPoints)
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDepth := stepDepth - 1
		for range countPoints {
			depth := currDepth
			benchCases = append(benchCases, benchCase{
				name: fmt.Sprintf("%s %s %d %d", opcode.REMOVE, typ, depth, 1),
				f: func() *VM {
					var item stackitem.Item
					if typ == stackitem.ArrayT {
						item = stackitem.NewArray(getArrayWithDepth(depth))
					} else {
						item = stackitem.NewMapWithValue([]stackitem.MapElement{
							{
								Key:   stackitem.NewBigInteger(big.NewInt(0)),
								Value: stackitem.NewArray(getArrayWithDepth(depth - 2)),
							},
						})
					}
					v := opParamPushVM(benchCase{op: opcode.REMOVE})()
					v.estack.PushVal(item)
					v.estack.PushVal(0)
					return v
				},
			})
			currDepth += stepDepth
		}
		if typ == stackitem.MapT {
			currN := stepN - 1
			for range countPoints {
				n := currN
				benchCases = append(benchCases, benchCase{
					name: fmt.Sprintf("%s %s %d %d", opcode.REMOVE, typ, 1, currN),
					f: func() *VM {
						keys := getUniqueKeys(n, stackitem.MaxKeySize)
						m := make([]stackitem.MapElement, 0, n)
						for _, key := range keys {
							m = append(m, stackitem.MapElement{Key: stackitem.NewByteArray(key)})
						}
						m[n-1].Value = stackitem.Null{}
						v := opParamPushVM(benchCase{op: opcode.REMOVE})()
						v.estack.PushVal(stackitem.NewMapWithValue(m))
						v.estack.PushVal(keys[n-1])
						return v
					},
				})
				currN += stepN
			}
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_CLEARITEMS(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 4*countPoints)
		stepDepth  = MaxStackSize / countPoints
		stepN      = MaxStackSize / countPoints
	)
	for _, typ := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		currDepth := stepDepth - 1
		for range countPoints {
			depth := currDepth
			benchCases = append(benchCases, benchCase{
				name: fmt.Sprintf("%s %s %d %d", opcode.CLEARITEMS, typ, depth, 1),
				f: func() *VM {
					var item stackitem.Item
					if typ == stackitem.ArrayT {
						item = stackitem.NewArray(getArrayWithDepth(depth))
					} else {
						item = stackitem.NewMapWithValue([]stackitem.MapElement{
							{
								Key:   stackitem.NewBigInteger(big.NewInt(0)),
								Value: stackitem.NewArray(getArrayWithDepth(depth - 2)),
							},
						})
					}
					v := opParamPushVM(benchCase{op: opcode.CLEARITEMS})()
					v.estack.PushVal(item)
					return v
				},
			})
			currDepth += stepDepth
		}
		currN := stepN - 1
		for range countPoints {
			n := currN
			benchCases = append(benchCases, benchCase{
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
					v := opParamPushVM(benchCase{op: opcode.CLEARITEMS})()
					v.estack.PushVal(item)
					return v
				},
			})
			currN += stepN
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_POPITEM(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, countPoints)
		stepDepth  = MaxStackSize / countPoints
		currDepth  = stepDepth - 1
	)
	for range countPoints {
		depth := currDepth
		benchCases = append(benchCases, benchCase{
			name: fmt.Sprintf("%s %s %d", opcode.POPITEM, stackitem.ArrayT, depth),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.POPITEM})()
				v.estack.PushVal(stackitem.NewArray(getArrayWithDepth(depth)))
				return v
			},
		})
		currDepth += stepDepth
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_SIZE(b *testing.B) {
	benchCases := []benchCase{
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
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_JUMP(b *testing.B) {
	benchCases := []benchCase{
		{
			op:     opcode.JMP,
			params: []byte{0},
		},
		{
			op:     opcode.JMPIF,
			params: []byte{0},
			items:  []any{true},
		},
		{
			op:     opcode.JMPEQ,
			params: []byte{0},
			items:  []any{0, 0},
		},
	}
	for _, bc := range benchCases {
		b.Run(bc.op.String(), func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_CALL(b *testing.B) {
	benchCases := []benchCase{
		{
			name: opcode.CALL.String(),
			f: func() *VM {
				return opParamPushVM(benchCase{op: opcode.CALL, params: []byte{0}})()
			},
		},
		{
			name: opcode.CALLA.String(),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.CALLA})()
				p := stackitem.NewPointerWithHash(0, []byte{42}, v.Context().ScriptHash())
				v.estack.PushVal(p)
				return v
			},
		},
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_KEYS(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 2*countPoints)
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
			benchCases = append(benchCases, benchCase{
				op:    opcode.KEYS,
				name:  fmt.Sprintf("%s %s %d", opcode.KEYS, typ, currMapLen),
				items: []any{stackitem.NewMapWithValue(m)},
			})
			currMapLen += stepMapLen
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_VALUES(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 6*countPoints)
		stepDepth  = MaxStackSize / countPoints
		stepN      = MaxStackSize / countPoints
	)
	for _, itemTyp := range []stackitem.Type{stackitem.ArrayT, stackitem.MapT} {
		for _, clonedTyp := range []stackitem.Type{stackitem.AnyT, stackitem.StructT} {
			currDepth := stepDepth - 1
			for range countPoints {
				var item, clonedItem stackitem.Item
				if clonedTyp == stackitem.StructT {
					clonedItem = stackitem.NewStruct(getStructWithDepth(currDepth - 1))
				} else {
					clonedItem = stackitem.NewArray(getArrayWithDepth(currDepth - 1))
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
				benchCases = append(benchCases, benchCase{
					name:  fmt.Sprintf("%s %s %s %d %d", opcode.VALUES, itemTyp, clonedTyp, currDepth, 1),
					items: []any{item},
				})
				currDepth += stepDepth
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
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %s %d %d", opcode.VALUES, itemTyp, stackitem.AnyT, n, n),
				items: []any{item},
			})
			currN += stepN
		}
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_RET(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 2*countPoints)
		stepDepth  = MaxStackSize / countPoints
		stepN      = MaxStackSize / countPoints
		currDepth  = stepDepth
		currN      = stepN
	)
	for range countPoints {
		depth := currDepth
		benchCases = append(benchCases, benchCase{
			name: fmt.Sprintf("%s %d %d", opcode.RET, depth, 1),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.RET})()
				callerCtx := &Context{retCount: 1, sc: &scriptContext{estack: NewStack("call")}}
				callerCtx.sc.estack.PushVal(stackitem.NewArray(getArrayWithDepth(depth - 1)))
				v.istack = append(v.istack, callerCtx)
				v.estack = callerCtx.Estack()
				return v
			},
		})
		currDepth += stepDepth
	}
	for range countPoints {
		n := currN
		benchCases = append(benchCases, benchCase{
			name: fmt.Sprintf("%s %d %d", opcode.RET, n, n),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.RET})()
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
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_HASKEY(b *testing.B) {
	var (
		benchCases = make([]benchCase, 0, 2*countPoints+2)
		stepN      = MaxStackSize / (2 * countPoints)
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
			benchCases = append(benchCases, benchCase{
				name:  fmt.Sprintf("%s %s %d %s", opcode.HASKEY, stackitem.MapT, currN, typ),
				items: []any{stackitem.NewMapWithValue(m), m[currN-1].Key},
			})
			currN += stepN
		}
	}
	benchCases = append(benchCases, benchCase{
		name:  fmt.Sprintf("%s %s", opcode.HASKEY, stackitem.ArrayT),
		items: []any{[]any{}, 0},
	})
	benchCases = append(benchCases, benchCase{
		name:  fmt.Sprintf("%s %s", opcode.HASKEY, stackitem.ByteArrayT),
		items: []any{[]byte{}, 0},
	})
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, opParamPushVM(bc))
		})
	}
}

func Benchmark_THROW_ABORT(b *testing.B) {
	benchCases := []benchCase{
		{
			name:  opcode.THROW.String(),
			op:    opcode.THROW,
			items: []any{"error"},
		},
		{
			name: opcode.ABORT.String(),
			op:   opcode.ABORT,
		},
		{
			name:  opcode.ABORTMSG.String(),
			op:    opcode.ABORTMSG,
			items: []any{"error"},
		},
		{
			name:  opcode.ASSERT.String(),
			op:    opcode.ASSERT,
			items: []any{false},
		},
		{
			name:  opcode.ASSERTMSG.String(),
			op:    opcode.ASSERTMSG,
			items: []any{false, "error"},
		},
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, exceptParamPushVM(bc.op, nil, bc.items...))
		})
	}
}

func Benchmark_TRY(b *testing.B) {
	benchCases := []benchCase{
		{
			name: opcode.TRY.String(),
			f: func() *VM {
				return opParamPushVM(benchCase{op: opcode.TRY, params: []byte{1, 0}})()
			},
		},
		{
			name: opcode.ENDTRY.String(),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.ENDTRY, params: []byte{0}})()
				v.Context().tryStack.PushVal(newExceptionHandlingContext(0, -1))
				return v
			},
		},
		{
			name: opcode.ENDFINALLY.String(),
			f: func() *VM {
				v := opParamPushVM(benchCase{op: opcode.ENDFINALLY})()
				v.Context().tryStack.PushVal(&exceptionHandlingContext{EndOffset: 0})
				return v
			},
		},
	}
	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			benchOpcode(b, bc.f)
		})
	}
}

func Benchmark_OPCODES_WITHOUT_ANYTHING(b *testing.B) {
	opcodes := []opcode.Opcode{
		opcode.PUSHM1,
		opcode.PUSH0,
		opcode.PUSHT,
		opcode.PUSHNULL,
		opcode.NEWMAP,
		opcode.NOP,
	}
	for _, op := range opcodes {
		b.Run(op.String(), func(b *testing.B) {
			benchOpcode(b, opParamPushVM(benchCase{op: op}))
		})
	}
}
