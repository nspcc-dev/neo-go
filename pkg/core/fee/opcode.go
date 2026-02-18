package fee

import (
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"gonum.org/v1/gonum/floats"
)

const (
	nopNS = 81
)

var (
	appendNotRefW       = []float64{17.83877335, 7.27458653, 11.53347086, 873.81450894}
	appendRefW          = []float64{18.31925134, 14.7508451, 11.87780883, 124.9843999}
	catW                = []float64{3.69913231e-01, 6.84925871e+02}
	clearW              = []float64{7.51584427, -1.71256636, 249.94062962}
	clearItemsW         = []float64{7.26838231, 81.58101565}
	convertArrOrStructW = []float64{23.66101563, 267.30101563}
	convertByteArrayW   = []float64{4.10394456e-01, 1.42552347e+03}
	dropW               = []float64{7.5673601, 157.08139535}
	dupW                = []float64{4.07258562e-01, 1.87688302e+03}
	hasKeyW             = []float64{16.94132124, 168.19963717}
	initSlotW           = []float64{32.58349642, 111.33750401}
	keysW               = []float64{112.97893297, 686.04870041}
	memcpyW             = []float64{4.51368336e-02, 2.69825581e+02}
	newArrayByteOrIntW  = []float64{72.52015093, 315.20172481}
	newArrayAnyW        = []float64{15.23973474, 691.03043241}
	newBuffer           = []float64{0.4092797, 83.43145839}
	packW               = []float64{12.2476721, 9.74111449, 479.87706721}
	packMapW            = []float64{10.21371457, 8.55493437, 2424.88841189}
	pickItemAnyW        = []float64{5.75778857, 18.44609763, 6.82305305, 276.0629743}
	pickItemByteArrayW  = []float64{5.99784686, 18.87873391, 0.48617825, 159.13755199}
	popItemW            = []float64{7.56705518, 4.88905775, 178.38691174}
	removeW             = []float64{7.56387863, 7.35527533, 18.38937335, 184.77596451}
	reverseItemsArrayW  = []float64{7.56222604, 423.97009387}
	reverseItemsBufferW = []float64{2.86847869e-01, 2.58156218e+03}
	reverseW            = []float64{1.45780342, 187.19036156}
	rollW               = []float64{0.44331395, 137.0494186}
	setitemW            = []float64{7.48170618, 7.1382109, 12.36046887, 36.48021068, 457.45986421}
	sizeW               = []float64{7.50488495, 263.13827196}
	stW                 = []float64{6.89534043, 14.22715315, 52.47827189}
	substrW             = []float64{3.73879475e-01, 1.51875581e+03}
	unpackAnyW          = []float64{12.02552859, 7.49327755, 545.93718883}
	unpackMapW          = []float64{12.43006326, 6.23912533, 528.82044153}
	valuesW             = []float64{-7.58588745, 19.80260206, -0.96627645, 475.14541493}
	xDropW              = []float64{7.65992573, 0.46298167, 142.53077978}
)

// Opcode returns the deployment coefficients of the specified opcodes.
func Opcode(base int64, opcodes ...opcode.Opcode) int64 {
	var result int64
	for _, op := range opcodes {
		result += int64(coefficients[op])
	}
	return result * base
}

var coefficients = [256]uint16{
	opcode.PUSHINT8:     1 << 0,
	opcode.PUSHINT16:    1 << 0,
	opcode.PUSHINT32:    1 << 0,
	opcode.PUSHINT64:    1 << 0,
	opcode.PUSHINT128:   1 << 2,
	opcode.PUSHINT256:   1 << 2,
	opcode.PUSHT:        1 << 0,
	opcode.PUSHF:        1 << 0,
	opcode.PUSHA:        1 << 2,
	opcode.PUSHNULL:     1 << 0,
	opcode.PUSHDATA1:    1 << 3,
	opcode.PUSHDATA2:    1 << 9,
	opcode.PUSHDATA4:    1 << 12,
	opcode.PUSHM1:       1 << 0,
	opcode.PUSH0:        1 << 0,
	opcode.PUSH1:        1 << 0,
	opcode.PUSH2:        1 << 0,
	opcode.PUSH3:        1 << 0,
	opcode.PUSH4:        1 << 0,
	opcode.PUSH5:        1 << 0,
	opcode.PUSH6:        1 << 0,
	opcode.PUSH7:        1 << 0,
	opcode.PUSH8:        1 << 0,
	opcode.PUSH9:        1 << 0,
	opcode.PUSH10:       1 << 0,
	opcode.PUSH11:       1 << 0,
	opcode.PUSH12:       1 << 0,
	opcode.PUSH13:       1 << 0,
	opcode.PUSH14:       1 << 0,
	opcode.PUSH15:       1 << 0,
	opcode.PUSH16:       1 << 0,
	opcode.NOP:          1 << 0,
	opcode.JMP:          1 << 1,
	opcode.JMPL:         1 << 1,
	opcode.JMPIF:        1 << 1,
	opcode.JMPIFL:       1 << 1,
	opcode.JMPIFNOT:     1 << 1,
	opcode.JMPIFNOTL:    1 << 1,
	opcode.JMPEQ:        1 << 1,
	opcode.JMPEQL:       1 << 1,
	opcode.JMPNE:        1 << 1,
	opcode.JMPNEL:       1 << 1,
	opcode.JMPGT:        1 << 1,
	opcode.JMPGTL:       1 << 1,
	opcode.JMPGE:        1 << 1,
	opcode.JMPGEL:       1 << 1,
	opcode.JMPLT:        1 << 1,
	opcode.JMPLTL:       1 << 1,
	opcode.JMPLE:        1 << 1,
	opcode.JMPLEL:       1 << 1,
	opcode.CALL:         1 << 9,
	opcode.CALLL:        1 << 9,
	opcode.CALLA:        1 << 9,
	opcode.CALLT:        1 << 15,
	opcode.ABORT:        0,
	opcode.ASSERT:       1 << 0,
	opcode.ABORTMSG:     0,
	opcode.ASSERTMSG:    1 << 0,
	opcode.THROW:        1 << 9,
	opcode.TRY:          1 << 2,
	opcode.TRYL:         1 << 2,
	opcode.ENDTRY:       1 << 2,
	opcode.ENDTRYL:      1 << 2,
	opcode.ENDFINALLY:   1 << 2,
	opcode.RET:          0,
	opcode.SYSCALL:      0,
	opcode.DEPTH:        1 << 1,
	opcode.DROP:         1 << 1,
	opcode.NIP:          1 << 1,
	opcode.XDROP:        1 << 4,
	opcode.CLEAR:        1 << 4,
	opcode.DUP:          1 << 1,
	opcode.OVER:         1 << 1,
	opcode.PICK:         1 << 1,
	opcode.TUCK:         1 << 1,
	opcode.SWAP:         1 << 1,
	opcode.ROT:          1 << 1,
	opcode.ROLL:         1 << 4,
	opcode.REVERSE3:     1 << 1,
	opcode.REVERSE4:     1 << 1,
	opcode.REVERSEN:     1 << 4,
	opcode.INITSSLOT:    1 << 4,
	opcode.INITSLOT:     1 << 6,
	opcode.LDSFLD0:      1 << 1,
	opcode.LDSFLD1:      1 << 1,
	opcode.LDSFLD2:      1 << 1,
	opcode.LDSFLD3:      1 << 1,
	opcode.LDSFLD4:      1 << 1,
	opcode.LDSFLD5:      1 << 1,
	opcode.LDSFLD6:      1 << 1,
	opcode.LDSFLD:       1 << 1,
	opcode.STSFLD0:      1 << 1,
	opcode.STSFLD1:      1 << 1,
	opcode.STSFLD2:      1 << 1,
	opcode.STSFLD3:      1 << 1,
	opcode.STSFLD4:      1 << 1,
	opcode.STSFLD5:      1 << 1,
	opcode.STSFLD6:      1 << 1,
	opcode.STSFLD:       1 << 1,
	opcode.LDLOC0:       1 << 1,
	opcode.LDLOC1:       1 << 1,
	opcode.LDLOC2:       1 << 1,
	opcode.LDLOC3:       1 << 1,
	opcode.LDLOC4:       1 << 1,
	opcode.LDLOC5:       1 << 1,
	opcode.LDLOC6:       1 << 1,
	opcode.LDLOC:        1 << 1,
	opcode.STLOC0:       1 << 1,
	opcode.STLOC1:       1 << 1,
	opcode.STLOC2:       1 << 1,
	opcode.STLOC3:       1 << 1,
	opcode.STLOC4:       1 << 1,
	opcode.STLOC5:       1 << 1,
	opcode.STLOC6:       1 << 1,
	opcode.STLOC:        1 << 1,
	opcode.LDARG0:       1 << 1,
	opcode.LDARG1:       1 << 1,
	opcode.LDARG2:       1 << 1,
	opcode.LDARG3:       1 << 1,
	opcode.LDARG4:       1 << 1,
	opcode.LDARG5:       1 << 1,
	opcode.LDARG6:       1 << 1,
	opcode.LDARG:        1 << 1,
	opcode.STARG0:       1 << 1,
	opcode.STARG1:       1 << 1,
	opcode.STARG2:       1 << 1,
	opcode.STARG3:       1 << 1,
	opcode.STARG4:       1 << 1,
	opcode.STARG5:       1 << 1,
	opcode.STARG6:       1 << 1,
	opcode.STARG:        1 << 1,
	opcode.NEWBUFFER:    1 << 8,
	opcode.MEMCPY:       1 << 11,
	opcode.CAT:          1 << 11,
	opcode.SUBSTR:       1 << 11,
	opcode.LEFT:         1 << 11,
	opcode.RIGHT:        1 << 11,
	opcode.INVERT:       1 << 2,
	opcode.AND:          1 << 3,
	opcode.OR:           1 << 3,
	opcode.XOR:          1 << 3,
	opcode.EQUAL:        1 << 5,
	opcode.NOTEQUAL:     1 << 5,
	opcode.SIGN:         1 << 2,
	opcode.ABS:          1 << 2,
	opcode.NEGATE:       1 << 2,
	opcode.INC:          1 << 2,
	opcode.DEC:          1 << 2,
	opcode.ADD:          1 << 3,
	opcode.SUB:          1 << 3,
	opcode.MUL:          1 << 3,
	opcode.DIV:          1 << 3,
	opcode.MOD:          1 << 3,
	opcode.POW:          1 << 6,
	opcode.SQRT:         1 << 6,
	opcode.MODMUL:       1 << 5,
	opcode.MODPOW:       1 << 11,
	opcode.SHL:          1 << 3,
	opcode.SHR:          1 << 3,
	opcode.NOT:          1 << 2,
	opcode.BOOLAND:      1 << 3,
	opcode.BOOLOR:       1 << 3,
	opcode.NZ:           1 << 2,
	opcode.NUMEQUAL:     1 << 3,
	opcode.NUMNOTEQUAL:  1 << 3,
	opcode.LT:           1 << 3,
	opcode.LE:           1 << 3,
	opcode.GT:           1 << 3,
	opcode.GE:           1 << 3,
	opcode.MIN:          1 << 3,
	opcode.MAX:          1 << 3,
	opcode.WITHIN:       1 << 3,
	opcode.PACKMAP:      1 << 11,
	opcode.PACKSTRUCT:   1 << 11,
	opcode.PACK:         1 << 11,
	opcode.UNPACK:       1 << 11,
	opcode.NEWARRAY0:    1 << 4,
	opcode.NEWARRAY:     1 << 9,
	opcode.NEWARRAYT:    1 << 9,
	opcode.NEWSTRUCT0:   1 << 4,
	opcode.NEWSTRUCT:    1 << 9,
	opcode.NEWMAP:       1 << 3,
	opcode.SIZE:         1 << 2,
	opcode.HASKEY:       1 << 6,
	opcode.KEYS:         1 << 4,
	opcode.VALUES:       1 << 13,
	opcode.PICKITEM:     1 << 6,
	opcode.APPEND:       1 << 13,
	opcode.SETITEM:      1 << 13,
	opcode.REVERSEITEMS: 1 << 13,
	opcode.REMOVE:       1 << 4,
	opcode.CLEARITEMS:   1 << 4,
	opcode.POPITEM:      1 << 4,
	opcode.ISNULL:       1 << 1,
	opcode.ISTYPE:       1 << 1,
	opcode.CONVERT:      1 << 13,
}

func OpcodeStatic(base int64, op opcode.Opcode) int64 {
	return base * staticCoefficients[op]
}

var staticCoefficients = [256]int64{
	opcode.PUSHINT8:     3,
	opcode.PUSHINT16:    3,
	opcode.PUSHINT32:    3,
	opcode.PUSHINT64:    3,
	opcode.PUSHINT128:   3,
	opcode.PUSHINT256:   3,
	opcode.PUSHT:        1,
	opcode.PUSHF:        1,
	opcode.PUSHA:        2,
	opcode.PUSHNULL:     1,
	opcode.PUSHDATA1:    2,
	opcode.PUSHDATA2:    2,
	opcode.PUSHDATA4:    2,
	opcode.PUSHM1:       2,
	opcode.PUSH0:        2,
	opcode.PUSH1:        2,
	opcode.PUSH2:        2,
	opcode.PUSH3:        2,
	opcode.PUSH4:        2,
	opcode.PUSH5:        2,
	opcode.PUSH6:        2,
	opcode.PUSH7:        2,
	opcode.PUSH8:        2,
	opcode.PUSH9:        2,
	opcode.PUSH10:       2,
	opcode.PUSH11:       2,
	opcode.PUSH12:       2,
	opcode.PUSH13:       2,
	opcode.PUSH14:       2,
	opcode.PUSH15:       2,
	opcode.PUSH16:       2,
	opcode.NOP:          1,
	opcode.JMP:          1,
	opcode.JMPL:         1,
	opcode.JMPIF:        1,
	opcode.JMPIFL:       1,
	opcode.JMPIFNOT:     1,
	opcode.JMPIFNOTL:    1,
	opcode.JMPEQ:        2,
	opcode.JMPEQL:       2,
	opcode.JMPNE:        2,
	opcode.JMPNEL:       2,
	opcode.JMPGT:        2,
	opcode.JMPGTL:       2,
	opcode.JMPGE:        2,
	opcode.JMPGEL:       2,
	opcode.JMPLT:        2,
	opcode.JMPLTL:       2,
	opcode.JMPLE:        2,
	opcode.JMPLEL:       2,
	opcode.CALL:         3,
	opcode.CALLL:        3,
	opcode.CALLA:        3,
	opcode.CALLT:        1 << 15,
	opcode.ABORT:        0,
	opcode.ASSERT:       1,
	opcode.ABORTMSG:     0,
	opcode.ASSERTMSG:    2,
	opcode.THROW:        1 << 9,
	opcode.TRY:          2,
	opcode.TRYL:         2,
	opcode.ENDTRY:       1,
	opcode.ENDTRYL:      1,
	opcode.ENDFINALLY:   1,
	opcode.RET:          1,
	opcode.SYSCALL:      0,
	opcode.DEPTH:        3,
	opcode.DROP:         1,
	opcode.NIP:          1,
	opcode.XDROP:        1,
	opcode.CLEAR:        1,
	opcode.DUP:          1,
	opcode.OVER:         1,
	opcode.PICK:         1,
	opcode.TUCK:         1,
	opcode.SWAP:         1,
	opcode.ROT:          1,
	opcode.ROLL:         1,
	opcode.REVERSE3:     1,
	opcode.REVERSE4:     1,
	opcode.REVERSEN:     1,
	opcode.INITSSLOT:    1,
	opcode.INITSLOT:     1,
	opcode.LDSFLD0:      1,
	opcode.LDSFLD1:      1,
	opcode.LDSFLD2:      1,
	opcode.LDSFLD3:      1,
	opcode.LDSFLD4:      1,
	opcode.LDSFLD5:      1,
	opcode.LDSFLD6:      1,
	opcode.LDSFLD:       1,
	opcode.STSFLD0:      1,
	opcode.STSFLD1:      1,
	opcode.STSFLD2:      1,
	opcode.STSFLD3:      1,
	opcode.STSFLD4:      1,
	opcode.STSFLD5:      1,
	opcode.STSFLD6:      1,
	opcode.STSFLD:       1,
	opcode.LDLOC0:       1,
	opcode.LDLOC1:       1,
	opcode.LDLOC2:       1,
	opcode.LDLOC3:       1,
	opcode.LDLOC4:       1,
	opcode.LDLOC5:       1,
	opcode.LDLOC6:       1,
	opcode.LDLOC:        1,
	opcode.STLOC0:       1,
	opcode.STLOC1:       1,
	opcode.STLOC2:       1,
	opcode.STLOC3:       1,
	opcode.STLOC4:       1,
	opcode.STLOC5:       1,
	opcode.STLOC6:       1,
	opcode.STLOC:        1,
	opcode.LDARG0:       1,
	opcode.LDARG1:       1,
	opcode.LDARG2:       1,
	opcode.LDARG3:       1,
	opcode.LDARG4:       1,
	opcode.LDARG5:       1,
	opcode.LDARG6:       1,
	opcode.LDARG:        1,
	opcode.STARG0:       1,
	opcode.STARG1:       1,
	opcode.STARG2:       1,
	opcode.STARG3:       1,
	opcode.STARG4:       1,
	opcode.STARG5:       1,
	opcode.STARG6:       1,
	opcode.STARG:        1,
	opcode.NEWBUFFER:    1,
	opcode.MEMCPY:       1,
	opcode.CAT:          1,
	opcode.SUBSTR:       1,
	opcode.LEFT:         1,
	opcode.RIGHT:        1,
	opcode.INVERT:       3,
	opcode.AND:          3,
	opcode.OR:           3,
	opcode.XOR:          3,
	opcode.EQUAL:        2,
	opcode.NOTEQUAL:     2,
	opcode.SIGN:         3,
	opcode.ABS:          3,
	opcode.NEGATE:       3,
	opcode.INC:          3,
	opcode.DEC:          3,
	opcode.ADD:          3,
	opcode.SUB:          3,
	opcode.MUL:          4,
	opcode.DIV:          4,
	opcode.MOD:          4,
	opcode.POW:          6,
	opcode.SQRT:         8,
	opcode.MODMUL:       4,
	opcode.MODPOW:       8,
	opcode.SHL:          4,
	opcode.SHR:          3,
	opcode.NOT:          2,
	opcode.BOOLAND:      2,
	opcode.BOOLOR:       2,
	opcode.NZ:           2,
	opcode.NUMEQUAL:     3,
	opcode.NUMNOTEQUAL:  2,
	opcode.LT:           2,
	opcode.LE:           1,
	opcode.GT:           1,
	opcode.GE:           1,
	opcode.MIN:          2,
	opcode.MAX:          2,
	opcode.WITHIN:       2,
	opcode.PACKMAP:      1,
	opcode.PACKSTRUCT:   1,
	opcode.PACK:         1,
	opcode.UNPACK:       1,
	opcode.NEWARRAY0:    2,
	opcode.NEWARRAY:     1,
	opcode.NEWARRAYT:    1,
	opcode.NEWSTRUCT0:   2,
	opcode.NEWSTRUCT:    1,
	opcode.NEWMAP:       2,
	opcode.SIZE:         1,
	opcode.HASKEY:       1,
	opcode.KEYS:         1,
	opcode.VALUES:       1,
	opcode.PICKITEM:     1,
	opcode.APPEND:       1,
	opcode.SETITEM:      1,
	opcode.REVERSEITEMS: 1,
	opcode.REMOVE:       1,
	opcode.CLEARITEMS:   1,
	opcode.POPITEM:      1,
	opcode.ISNULL:       1,
	opcode.ISTYPE:       1,
	opcode.CONVERT:      1,
}

func OpcodeDynamic(base int64, op opcode.Opcode, args ...any) int64 {
	if dynamicCoefficients[op] == nil {
		fmt.Println(op, args)
	}
	return base * max(1, int64(dynamicCoefficients[op](args...)/nopNS))
}

var dynamicCoefficients = [256]func(args ...any) float64{
	opcode.APPEND:       AppendGas,
	opcode.CAT:          CatGas,
	opcode.CLEAR:        ClearGas,
	opcode.CLEARITEMS:   ClearItemsGas,
	opcode.CONVERT:      ConvertGas,
	opcode.DROP:         DropGas,
	opcode.DUP:          DupGas,
	opcode.HASKEY:       HasKeyGas,
	opcode.INITSLOT:     InitSlotGas,
	opcode.KEYS:         KeysGas,
	opcode.LEFT:         SubstrGas,
	opcode.MEMCPY:       MemcpyGas,
	opcode.NEWARRAY:     NewArrayGas,
	opcode.NEWARRAYT:    NewArrayGas,
	opcode.NEWBUFFER:    NewBufferGas,
	opcode.NEWSTRUCT:    NewArrayGas,
	opcode.NIP:          DropGas,
	opcode.OVER:         DupGas,
	opcode.PACK:         PackGas,
	opcode.PACKMAP:      PackMapGas,
	opcode.PACKSTRUCT:   PackGas,
	opcode.PICK:         DupGas,
	opcode.PICKITEM:     PickItemGas,
	opcode.POPITEM:      PopItemGas,
	opcode.REMOVE:       RemoveGas,
	opcode.REVERSEITEMS: ReverseItemsGas,
	opcode.REVERSE3:     ReverseGas,
	opcode.REVERSE4:     ReverseGas,
	opcode.REVERSEN:     ReverseGas,
	opcode.RIGHT:        SubstrGas,
	opcode.ROLL:         RollGas,
	opcode.ROT:          RollGas,
	opcode.SETITEM:      SetItemGas,
	opcode.SIZE:         SizeGas,
	opcode.STSFLD0:      STGas,
	opcode.STSFLD1:      STGas,
	opcode.STSFLD2:      STGas,
	opcode.STSFLD3:      STGas,
	opcode.STSFLD4:      STGas,
	opcode.STSFLD5:      STGas,
	opcode.STSFLD6:      STGas,
	opcode.STSFLD:       STGas,
	opcode.STLOC0:       STGas,
	opcode.STLOC1:       STGas,
	opcode.STLOC2:       STGas,
	opcode.STLOC3:       STGas,
	opcode.STLOC4:       STGas,
	opcode.STLOC5:       STGas,
	opcode.STLOC6:       STGas,
	opcode.STLOC:        STGas,
	opcode.STARG0:       STGas,
	opcode.STARG1:       STGas,
	opcode.STARG2:       STGas,
	opcode.STARG3:       STGas,
	opcode.STARG4:       STGas,
	opcode.STARG5:       STGas,
	opcode.STARG6:       STGas,
	opcode.STARG:        STGas,
	opcode.SUBSTR:       SubstrGas,
	opcode.UNPACK:       UnpackGas,
	opcode.VALUES:       ValuesGas,
	opcode.XDROP:        XDropGas,
}

func AppendGas(args ...any) float64 {
	if len(args) != 4 {
		panic("invalid arg len")
	}
	var (
		f   = args[0].(bool)
		ar  = args[1].(int)
		ir  = args[2].(int)
		nci = args[3].(int)
	)
	if f {
		return floats.Dot(appendRefW, []float64{float64(ar), float64(ir), float64(nci), 1})
	}
	return floats.Dot(appendNotRefW, []float64{float64(ar), float64(ir), float64(nci), 1})
}

func CatGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	l := args[0].(int)
	return floats.Dot(catW, []float64{float64(l), 1})
}

func ClearGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	r := args[0].(int)
	n := args[1].(int)
	return floats.Dot(clearW, []float64{float64(r), float64(n), 1})
}

func ClearItemsGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	r := args[0].(int)
	return floats.Dot(clearItemsW, []float64{float64(r), 1})
}

func ConvertGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		l = args[1].(int)
	)
	if t == stackitem.ArrayT {
		return floats.Dot(convertArrOrStructW, []float64{float64(l), 1})
	}
	return floats.Dot(convertByteArrayW, []float64{float64(l), 1})
}

func DropGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	r := args[0].(int)
	return floats.Dot(dropW, []float64{float64(r), 1})
}

func DupGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	l := args[0].(int)
	return floats.Dot(dupW, []float64{float64(l), 1})
}

func HasKeyGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	l := args[0].(int)
	return floats.Dot(hasKeyW, []float64{float64(l), 1})
}

func InitSlotGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	as := args[0].(int)
	return floats.Dot(initSlotW, []float64{float64(as), 1})
}

func KeysGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	l := args[0].(int)
	return floats.Dot(keysW, []float64{float64(l), 1})
}

func MemcpyGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	l := args[0].(int)
	return floats.Dot(memcpyW, []float64{float64(l), 1})
}

func NewArrayGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		l = args[1].(int)
	)
	if t == stackitem.ByteArrayT || t == stackitem.IntegerT {
		return floats.Dot(newArrayByteOrIntW, []float64{float64(l), 1})
	}
	return floats.Dot(newArrayAnyW, []float64{float64(l), 1})
}

func NewBufferGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	l := args[0].(int)
	return floats.Dot(newBuffer, []float64{float64(l), 1})
}

func PackGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		r = args[0].(int)
		l = args[1].(int)
	)
	return floats.Dot(packW, []float64{float64(r), float64(l), 1})
}

func PackMapGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		r  = args[0].(int)
		ss = args[1].(int)
	)
	return floats.Dot(packMapW, []float64{float64(r), float64(ss), 1})
}

func PickItemGas(args ...any) float64 {
	if len(args) != 4 {
		panic("invalid arg len")
	}
	var (
		it = args[0].(stackitem.Type)
		ar = args[1].(int)
		in = args[2].(int)
		is = args[3].(int)
	)
	if it == stackitem.ByteArrayT {
		return floats.Dot(pickItemByteArrayW, []float64{float64(ar), float64(in), float64(is), 1})
	}
	return floats.Dot(pickItemAnyW, []float64{float64(ar), float64(in), float64(is), 1})
}

func PopItemGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		ar = args[0].(int)
		ir = args[1].(int)
	)
	return floats.Dot(popItemW, []float64{float64(ar), float64(ir), 1})
}

func RemoveGas(args ...any) float64 {
	if len(args) != 3 {
		panic("invalid arg len")
	}
	var (
		ar = args[0].(int)
		ir = args[1].(int)
		in = args[2].(int)
	)
	return floats.Dot(removeW, []float64{float64(ar), float64(ir), float64(in), 1})
}

func ReverseItemsGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		n = args[1].(int)
	)
	if t == stackitem.BufferT {
		return floats.Dot(reverseItemsBufferW, []float64{float64(n), 1})
	}
	return floats.Dot(reverseItemsArrayW, []float64{float64(n), 1})
}

func ReverseGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	n := args[0].(int)
	return floats.Dot(reverseW, []float64{float64(n), 1})
}

func RollGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	n := args[0].(int)
	return floats.Dot(rollW, []float64{float64(n), 1})
}

func SetItemGas(args ...any) float64 {
	if len(args) != 4 {
		panic("invalid arg len")
	}
	var (
		ar   = args[0].(int)
		remr = args[1].(int)
		addr = args[2].(int)
		in   = args[3].(int)
	)
	return floats.Dot(setitemW, []float64{float64(ar), float64(remr), float64(addr), float64(in), 1})
}

func SizeGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	r := args[1].(int)
	return floats.Dot(sizeW, []float64{float64(r), 1})
}

func STGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		rr = args[0].(int)
		ar = args[1].(int)
	)
	return floats.Dot(stW, []float64{float64(rr), float64(ar), 1})
}

func SubstrGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	l := args[0].(int)
	return floats.Dot(substrW, []float64{float64(l), 1})
}

func UnpackGas(args ...any) float64 {
	if len(args) != 3 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		r = args[1].(int)
		l = args[2].(int)
	)
	if t == stackitem.MapT {
		return floats.Dot(unpackMapW, []float64{float64(r), float64(l), 1})
	}
	return floats.Dot(unpackAnyW, []float64{float64(r), float64(l), 1})
}

func ValuesGas(args ...any) float64 {
	if len(args) != 3 {
		panic("invalid arg len")
	}
	var (
		ar = args[0].(int)
		vr = args[1].(int)
		n  = args[2].(int)
	)
	return floats.Dot(valuesW, []float64{float64(ar), float64(vr), float64(n), 1})
}

func XDropGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		r = args[0].(int)
		n = args[1].(int)
	)
	return floats.Dot(xDropW, []float64{float64(r), float64(n), 1})
}
