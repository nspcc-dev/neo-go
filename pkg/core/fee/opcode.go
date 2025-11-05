package fee

import (
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"gonum.org/v1/gonum/floats"
)

const (
	nopNS = 111.173603
)

var (
	appendAnyW            = []float64{11.73069586, 350.41674237}
	appendStructW         = []float64{87.30446039, 17982.61841388}
	catW                  = []float64{4.91009541e-01, -6.22930233e+02}
	clearW                = []float64{5.67140448, -0.73312282, 298.39230503}
	clearItemsW           = []float64{5.28724817, 1.88144976, 516.52124099}
	convertArrayW         = []float64{11.60835938, 1192.48835938}
	convertByteArrayW     = []float64{1.35404383e-01, 1.35296000e+03}
	dropW                 = []float64{4.75086301, 1772.40697674}
	dupW                  = []float64{8.63749860e-02, 3.62736047e+03}
	hasKeyIntegerW        = []float64{52.79587573, -891.9134266}
	hasKeyByteArrayW      = []float64{58.0377907, -3630.68313953}
	initSlotW             = []float64{2.64587955, 20.75369237, 470.12611159}
	keysByteArrayW        = []float64{130, 37481}
	keysIntegerW          = []float64{123, 19510}
	memcpyW               = []float64{4.51368336e-02, 2.69825581e+02}
	newArrayByteArrayW    = []float64{71.78479288, 1277.08711846}
	newArrayAnyW          = []float64{15.23973474, 691.03043241}
	newBuffer             = []float64{3.78677415e-01, 2.63129070e+03}
	packW                 = []float64{10.77058596, 6.29235395, 1629.89894874}
	packMapIntegerW       = []float64{1.28644848e+02, 8.83537843e+00, -1.74640868e+05}
	packMapByteArrayW     = []float64{3.90458381e+01, 9.57439349e+00, -3.69134485e+04}
	pickItemAnyAnyW       = []float64{11.35446948, 525.93586483}
	pickItemAnyByteArrayW = []float64{2.13154163e-01, 4.04324804e+03}
	pickItemMapAnyW       = []float64{29.57506086, 16.10914509, -5696.71848299}
	pickItemMapByteArrayW = []float64{2.58620558e+01, 2.56974509e-01, 4.87687014e+02}
	popItemW              = []float64{10.72283794, 699.34883721}
	removeArrayW          = []float64{5.10601381, 1097.15252544}
	removeMapW            = []float64{5.41334121, 10.31069302, 993.41112468}
	retW                  = []float64{4.87054774e+00, 9.08958062e-02, 1.05882523e+04}
	reverseItemsArrayW    = []float64{8.30759448, -529.41860465}
	reverseItemsBufferW   = []float64{5.42999763e+00, 3.48448837e+04}
	reverseW              = []float64{1.45780342, 187.19036156}
	rollW                 = []float64{0.44331395, 137.0494186}
	setitemAnyW           = []float64{5.4464, 10.61103203, 770.70123528}
	setitemMapW           = []float64{5.14137346, 10.61948722, 32.23029414, 1426.78225675}
	stW                   = []float64{5.8727105, 11.69145319, 131.27384571}
	substrW               = []float64{5.25530577e-01, -3.61574419e+03}
	unpackAnyW            = []float64{12.42062092, 36.69888887, 686.75638358}
	unpackMapW            = []float64{18.7980629, 61.54088073, -9239.49545835}
	valuesAnyAnyW         = []float64{10.15299821, 7.60633215, 2549.89349556}
	valuesAnyStructW      = []float64{94.83527304, -78.29029852, 4157.9709503}
	valuesMapAnyW         = []float64{15.9299167, 12.9647698, -5187.57312918}
	valuesMapStructW      = []float64{99.34192445, -71.5646272, -3707.90104081}
	xDropW                = []float64{7.65992573, 0.46298167, 142.53077978}
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
	opcode.PUSHINT8:     1 << 2,
	opcode.PUSHINT16:    1 << 2,
	opcode.PUSHINT32:    1 << 2,
	opcode.PUSHINT64:    1 << 2,
	opcode.PUSHINT128:   1 << 2,
	opcode.PUSHINT256:   1 << 2,
	opcode.PUSHT:        1 << 0,
	opcode.PUSHF:        1 << 0,
	opcode.PUSHA:        1 << 2,
	opcode.PUSHNULL:     1 << 0,
	opcode.PUSHDATA1:    3,
	opcode.PUSHDATA2:    3,
	opcode.PUSHDATA4:    3,
	opcode.PUSHM1:       3,
	opcode.PUSH0:        3,
	opcode.PUSH1:        3,
	opcode.PUSH2:        3,
	opcode.PUSH3:        3,
	opcode.PUSH4:        3,
	opcode.PUSH5:        3,
	opcode.PUSH6:        3,
	opcode.PUSH7:        3,
	opcode.PUSH8:        3,
	opcode.PUSH9:        3,
	opcode.PUSH10:       3,
	opcode.PUSH11:       3,
	opcode.PUSH12:       3,
	opcode.PUSH13:       3,
	opcode.PUSH14:       3,
	opcode.PUSH15:       3,
	opcode.PUSH16:       3,
	opcode.NOP:          1 << 0,
	opcode.JMP:          1 << 0,
	opcode.JMPL:         1 << 0,
	opcode.JMPIF:        1 << 0,
	opcode.JMPIFL:       1 << 0,
	opcode.JMPIFNOT:     1 << 0,
	opcode.JMPIFNOTL:    1 << 0,
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
	opcode.CALL:         3,
	opcode.CALLL:        3,
	opcode.CALLA:        3,
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
	opcode.DEPTH:        1 << 0,
	opcode.DROP:         1 << 0,
	opcode.NIP:          1 << 0,
	opcode.XDROP:        1 << 0,
	opcode.CLEAR:        1 << 0,
	opcode.DUP:          1 << 0,
	opcode.OVER:         1 << 0,
	opcode.PICK:         1 << 0,
	opcode.TUCK:         1 << 0,
	opcode.SWAP:         1 << 1,
	opcode.ROT:          1 << 0,
	opcode.ROLL:         1 << 0,
	opcode.REVERSE3:     1 << 0,
	opcode.REVERSE4:     1 << 0,
	opcode.REVERSEN:     1 << 0,
	opcode.INITSSLOT:    1 << 0,
	opcode.INITSLOT:     1 << 0,
	opcode.LDSFLD0:      1 << 0,
	opcode.LDSFLD1:      1 << 0,
	opcode.LDSFLD2:      1 << 0,
	opcode.LDSFLD3:      1 << 0,
	opcode.LDSFLD4:      1 << 0,
	opcode.LDSFLD5:      1 << 0,
	opcode.LDSFLD6:      1 << 0,
	opcode.LDSFLD:       1 << 0,
	opcode.STSFLD0:      1 << 0,
	opcode.STSFLD1:      1 << 0,
	opcode.STSFLD2:      1 << 0,
	opcode.STSFLD3:      1 << 0,
	opcode.STSFLD4:      1 << 0,
	opcode.STSFLD5:      1 << 0,
	opcode.STSFLD6:      1 << 0,
	opcode.STSFLD:       1 << 0,
	opcode.LDLOC0:       1 << 0,
	opcode.LDLOC1:       1 << 0,
	opcode.LDLOC2:       1 << 0,
	opcode.LDLOC3:       1 << 0,
	opcode.LDLOC4:       1 << 0,
	opcode.LDLOC5:       1 << 0,
	opcode.LDLOC6:       1 << 0,
	opcode.LDLOC:        1 << 0,
	opcode.STLOC0:       1 << 0,
	opcode.STLOC1:       1 << 0,
	opcode.STLOC2:       1 << 0,
	opcode.STLOC3:       1 << 0,
	opcode.STLOC4:       1 << 0,
	opcode.STLOC5:       1 << 0,
	opcode.STLOC6:       1 << 0,
	opcode.STLOC:        1 << 0,
	opcode.LDARG0:       1 << 0,
	opcode.LDARG1:       1 << 0,
	opcode.LDARG2:       1 << 0,
	opcode.LDARG3:       1 << 0,
	opcode.LDARG4:       1 << 0,
	opcode.LDARG5:       1 << 0,
	opcode.LDARG6:       1 << 0,
	opcode.LDARG:        1 << 0,
	opcode.STARG0:       1 << 0,
	opcode.STARG1:       1 << 0,
	opcode.STARG2:       1 << 0,
	opcode.STARG3:       1 << 0,
	opcode.STARG4:       1 << 0,
	opcode.STARG5:       1 << 0,
	opcode.STARG6:       1 << 0,
	opcode.STARG:        1 << 0,
	opcode.NEWBUFFER:    1 << 2,
	opcode.MEMCPY:       1 << 0,
	opcode.CAT:          1 << 0,
	opcode.SUBSTR:       1 << 0,
	opcode.LEFT:         1 << 0,
	opcode.RIGHT:        1 << 0,
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
	opcode.DEC:          3,
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
	opcode.PACKMAP:      1 << 0,
	opcode.PACKSTRUCT:   1 << 0,
	opcode.PACK:         1 << 0,
	opcode.UNPACK:       1 << 0,
	opcode.NEWARRAY0:    1 << 4,
	opcode.NEWARRAY:     1 << 0,
	opcode.NEWARRAYT:    1 << 0,
	opcode.NEWSTRUCT0:   1 << 4,
	opcode.NEWSTRUCT:    1 << 0,
	opcode.NEWMAP:       1 << 3,
	opcode.SIZE:         1 << 2,
	opcode.HASKEY:       1 << 0,
	opcode.KEYS:         1 << 0,
	opcode.VALUES:       1 << 0,
	opcode.PICKITEM:     1 << 0,
	opcode.APPEND:       1 << 0,
	opcode.SETITEM:      1 << 0,
	opcode.REVERSEITEMS: 1 << 0,
	opcode.REMOVE:       1 << 0,
	opcode.CLEARITEMS:   1 << 0,
	opcode.POPITEM:      1 << 0,
	opcode.ISNULL:       1 << 1,
	opcode.ISTYPE:       1 << 1,
	opcode.CONVERT:      1 << 0,
}

func OpcodeDynamic(base int64, op opcode.Opcode, args ...any) int64 {
	return base * max(1, int64(dynamicPrices[op](args...)/nopNS))
}

var dynamicPrices = [256]func(args ...any) float64{
	opcode.APPEND:       AppendGas,
	opcode.CAT:          CatGas,
	opcode.CLEAR:        ClearGas,
	opcode.CLEARITEMS:   ClearItemsGas,
	opcode.CONVERT:      ConvertGas,
	opcode.DROP:         DropGas,
	opcode.DUP:          DupGas,
	opcode.HASKEY:       HasKeyGas,
	opcode.INITSLOT:     InitSlotGas,
	opcode.INITSSLOT:    InitSlotGas,
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
	opcode.PICKITEM:     PickItemGas,
	opcode.POPITEM:      PopItemGas,
	opcode.REMOVE:       RemoveGas,
	opcode.RET:          RetGas,
	opcode.REVERSEITEMS: ReverseItemsGas,
	opcode.REVERSE3:     ReverseGas,
	opcode.REVERSE4:     ReverseGas,
	opcode.REVERSEN:     ReverseGas,
	opcode.RIGHT:        SubstrGas,
	opcode.ROLL:         RollGas,
	opcode.ROT:          RollGas,
	opcode.SETITEM:      SetItemGas,
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
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		r = args[1].(int)
	)
	if t == stackitem.AnyT {
		return floats.Dot(appendAnyW, []float64{float64(r), 1})
	}
	return floats.Dot(appendStructW, []float64{float64(r), 1})
}

func CatGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	r := args[0].(int)
	return floats.Dot(catW, []float64{float64(r), 1})
}

func ClearGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	r := args[0].(int)
	s := args[1].(int)
	return floats.Dot(clearW, []float64{float64(r), float64(s), 1})
}

func ClearItemsGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		r = args[0].(int)
		l = args[1].(int)
	)
	return floats.Dot(clearItemsW, []float64{float64(r), float64(l), 1})
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
		return floats.Dot(convertArrayW, []float64{float64(l), 1})
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
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		l = args[1].(int)
	)
	if t == stackitem.ByteArrayT {
		return floats.Dot(hasKeyByteArrayW, []float64{float64(l), 1})
	}
	return floats.Dot(hasKeyIntegerW, []float64{float64(l), 1})
}

func InitSlotGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	l := args[0].(byte)
	a := args[1].(byte)
	return floats.Dot(initSlotW, []float64{float64(l), float64(a), 1})
}

func KeysGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		l = args[1].(int)
	)
	if t == stackitem.ByteArrayT {
		return floats.Dot(keysByteArrayW, []float64{float64(l), 1})
	}
	return floats.Dot(keysIntegerW, []float64{float64(l), 1})
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
		return floats.Dot(newArrayByteArrayW, []float64{float64(l), 1})
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
	if len(args) != 3 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		r = args[1].(int)
		l = args[2].(int)
	)
	if t == stackitem.ByteArrayT {
		return floats.Dot(packMapByteArrayW, []float64{float64(r), float64(l), 1})
	}
	return floats.Dot(packMapIntegerW, []float64{float64(r), float64(l), 1})
}

func PickItemGas(args ...any) float64 {
	if len(args) != 4 {
		panic("invalid arg len")
	}
	var (
		et = args[0].(stackitem.Type)
		it = args[1].(stackitem.Type)
		l  = args[2].(int)
		n  = args[3].(int)
	)
	if et == stackitem.MapT {
		if it == stackitem.ByteArrayT {
			return floats.Dot(pickItemMapByteArrayW, []float64{float64(l), float64(n), 1})
		}
		return floats.Dot(pickItemMapAnyW, []float64{float64(l), float64(n), 1})
	}
	if it == stackitem.ByteArrayT {
		return floats.Dot(pickItemAnyByteArrayW, []float64{float64(n), 1})
	}
	return floats.Dot(pickItemAnyAnyW, []float64{float64(n), 1})
}

func PopItemGas(args ...any) float64 {
	if len(args) != 1 {
		panic("invalid arg len")
	}
	r := args[0].(int)
	return floats.Dot(popItemW, []float64{float64(r), 1})
}

func RemoveGas(args ...any) float64 {
	if len(args) != 3 {
		panic("invalid arg len")
	}
	var (
		t = args[0].(stackitem.Type)
		r = args[1].(int)
		l = args[2].(int)
	)
	if t == stackitem.MapT {
		return floats.Dot(removeMapW, []float64{float64(r), float64(l), 1})
	}
	return floats.Dot(removeArrayW, []float64{float64(r), 1})
}

func RetGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	r := args[0].(int)
	s := args[1].(int)
	return floats.Dot(retW, []float64{float64(r), float64(s / 100), 1})
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
		t    = args[0].(stackitem.Type)
		l    = args[1].(int)
		rRem = args[2].(int)
		rAdd = args[3].(int)
	)
	if t == stackitem.MapT {
		return floats.Dot(setitemMapW, []float64{float64(rRem), float64(rAdd), float64(l), 1})
	}
	return floats.Dot(setitemAnyW, []float64{float64(rRem), float64(rAdd), 1})
}

func STGas(args ...any) float64 {
	if len(args) != 2 {
		panic("invalid arg len")
	}
	var (
		rRem = args[0].(int)
		rAdd = args[1].(int)
	)
	return floats.Dot(stW, []float64{float64(rRem), float64(rAdd), 1})
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
	if len(args) != 4 {
		panic("invalid arg len")
	}
	var (
		et = args[0].(stackitem.Type)
		at = args[1].(stackitem.Type)
		r  = args[2].(int)
		l  = args[3].(int)
	)
	if et == stackitem.MapT {
		if at == stackitem.StructT {
			return floats.Dot(valuesMapStructW, []float64{float64(r), float64(l), 1})
		}
		return floats.Dot(valuesMapAnyW, []float64{float64(r), float64(l), 1})
	}
	if at == stackitem.StructT {
		return floats.Dot(valuesAnyStructW, []float64{float64(r), float64(l), 1})
	}
	return floats.Dot(valuesAnyAnyW, []float64{float64(r), float64(l), 1})
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
