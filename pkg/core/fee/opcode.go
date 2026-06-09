package fee

import (
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// factor matches [vm.OpcodePriceMultiplier] and
// is used for fractional opcode pricing functionality.
const factor = 1000

// Opcode price weights used in dynamic fee formulas.
// All values are multiplied by [vm.OpcodePriceMultiplier].
var (
	appendW              = []int64{97, 192, 2715}
	assertW              = []int64{99, 9, 1847}
	catW                 = []int64{8, 2706}
	clearW               = []int64{93, -15, 1515}
	clearItemsW          = []int64{103, 1455}
	convertAnyW          = []int64{78, 1610}
	convertArrOrStructW  = []int64{77, 134, 3218}
	convertByteArrOrBufW = []int64{7, 3417}
	dropW                = []int64{99, 1486}
	endFinallyW          = []int64{93, 1137}
	hasKeyW              = []int64{99, 2575}
	initSlotW            = []int64{67, 160, 1702}
	isNullW              = []int64{97, 1236}
	isTypeW              = []int64{97, 1154}
	keysW                = []int64{96, 167, 2039}
	memCpyW              = []int64{1, 3514}
	newArrayAnyW         = []int64{126, 2718}
	newArrayByteOrIntW   = []int64{134, 2718}
	newBufferW           = []int64{6, 2507}
	packW                = []int64{173, 2649}
	packMapW             = []int64{80, 5281, 3481}
	pickItemW            = []int64{91, 2751}
	popItemW             = []int64{91, 3078}
	removeArrOrStructW   = []int64{98, 9, 1991}
	removeMapW           = []int64{96, 706, 6776}
	reverseItemsArrW     = []int64{98, 19, 2043}
	reverseItemsBufW     = []int64{9, 1690}
	reverseW             = []int64{19, 1702}
	rollW                = []int64{5, 1910}
	setItemW             = []int64{96, 149, 2390}
	sizeW                = []int64{100, 2693}
	stW                  = []int64{98, 1599}
	substrW              = []int64{7, 2908}
	throwW               = []int64{84, 1742}
	unpackW              = []int64{254, 2604}
	valuesW              = []int64{86, 312, 168, 4758}
	xDropW               = []int64{98, 6, 1791}
)

// Opcode returns the deployment coefficients of the specified opcodes.
func Opcode(base int64, opcodes ...opcode.Opcode) int64 {
	var result int64
	for _, op := range opcodes {
		result += coefficients[op]
	}
	return result * base
}

var coefficients = scaleCoefficients([256]int64{
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
}, factor)

func scaleCoefficients(src [256]int64, mul int64) [256]int64 {
	var dst [256]int64
	for i, c := range src {
		dst[i] = c * mul
	}
	return dst
}

// OpcodeV1 returns opcode pricing in the unit of Datoshi*[vm.OpcodePriceMultiplier]
// used since Gorgon hardfork, supporting dynamic (parameter-dependent) costs.
func OpcodeV1(base int64, op opcode.Opcode, params *vm.OpcodePriceParams) int64 {
	if dynamicCoefficients[op] != nil {
		return base * dynamicCoefficients[op](params)
	}
	return base * staticCoefficients[op]
}

var staticCoefficients = [256]int64{
	opcode.PUSHINT8:    2798,
	opcode.PUSHINT16:   2845,
	opcode.PUSHINT32:   2851,
	opcode.PUSHINT64:   2908,
	opcode.PUSHINT128:  3002,
	opcode.PUSHINT256:  3317,
	opcode.PUSHT:       1082,
	opcode.PUSHF:       1082,
	opcode.PUSHA:       2439,
	opcode.PUSHNULL:    1082,
	opcode.PUSHDATA1:   1685,
	opcode.PUSHDATA2:   1685,
	opcode.PUSHDATA4:   1685,
	opcode.PUSHM1:      2187,
	opcode.PUSH0:       2187,
	opcode.PUSH1:       2187,
	opcode.PUSH2:       2187,
	opcode.PUSH3:       2187,
	opcode.PUSH4:       2187,
	opcode.PUSH5:       2187,
	opcode.PUSH6:       2187,
	opcode.PUSH7:       2187,
	opcode.PUSH8:       2187,
	opcode.PUSH9:       2187,
	opcode.PUSH10:      2187,
	opcode.PUSH11:      2187,
	opcode.PUSH12:      2187,
	opcode.PUSH13:      2187,
	opcode.PUSH14:      2187,
	opcode.PUSH15:      2187,
	opcode.PUSH16:      2187,
	opcode.NOP:         1000,
	opcode.JMP:         1058,
	opcode.JMPL:        1058,
	opcode.JMPIF:       1211,
	opcode.JMPIFL:      1211,
	opcode.JMPIFNOT:    1211,
	opcode.JMPIFNOTL:   1211,
	opcode.JMPEQ:       1569,
	opcode.JMPEQL:      1569,
	opcode.JMPNE:       1569,
	opcode.JMPNEL:      1569,
	opcode.JMPGT:       1569,
	opcode.JMPGTL:      1569,
	opcode.JMPGE:       1569,
	opcode.JMPGEL:      1569,
	opcode.JMPLT:       1569,
	opcode.JMPLTL:      1569,
	opcode.JMPLE:       1569,
	opcode.JMPLEL:      1569,
	opcode.CALL:        2451,
	opcode.CALLL:       2451,
	opcode.CALLA:       2927,
	opcode.CALLT:       1 << 15,
	opcode.ABORT:       0,
	opcode.ABORTMSG:    0,
	opcode.TRY:         1753,
	opcode.TRYL:        1753,
	opcode.ENDTRY:      1127,
	opcode.ENDTRYL:     1127,
	opcode.RET:         1085,
	opcode.SYSCALL:     0,
	opcode.DEPTH:       2053,
	opcode.DUP:         1166,
	opcode.OVER:        1183,
	opcode.PICK:        1418,
	opcode.TUCK:        1379,
	opcode.SWAP:        1041,
	opcode.LDSFLD0:     1103,
	opcode.LDSFLD1:     1103,
	opcode.LDSFLD2:     1103,
	opcode.LDSFLD3:     1103,
	opcode.LDSFLD4:     1103,
	opcode.LDSFLD5:     1103,
	opcode.LDSFLD6:     1103,
	opcode.LDSFLD:      1103,
	opcode.LDLOC0:      1103,
	opcode.LDLOC1:      1103,
	opcode.LDLOC2:      1103,
	opcode.LDLOC3:      1103,
	opcode.LDLOC4:      1103,
	opcode.LDLOC5:      1103,
	opcode.LDLOC6:      1103,
	opcode.LDLOC:       1103,
	opcode.LDARG0:      1103,
	opcode.LDARG1:      1103,
	opcode.LDARG2:      1103,
	opcode.LDARG3:      1103,
	opcode.LDARG4:      1103,
	opcode.LDARG5:      1103,
	opcode.LDARG6:      1103,
	opcode.LDARG:       1103,
	opcode.INVERT:      3122,
	opcode.AND:         2967,
	opcode.OR:          3044,
	opcode.XOR:         3120,
	opcode.EQUAL:       1610,
	opcode.NOTEQUAL:    1610,
	opcode.SIGN:        2520,
	opcode.ABS:         2745,
	opcode.NEGATE:      2822,
	opcode.INC:         3099,
	opcode.DEC:         3053,
	opcode.ADD:         3153,
	opcode.SUB:         3233,
	opcode.MUL:         3541,
	opcode.DIV:         6459,
	opcode.MOD:         6522,
	opcode.POW:         8277,
	opcode.SQRT:        33802,
	opcode.MODMUL:      8136,
	opcode.MODPOW:      12300,
	opcode.SHL:         3711,
	opcode.SHR:         4315,
	opcode.NOT:         1262,
	opcode.BOOLAND:     1444,
	opcode.BOOLOR:      1476,
	opcode.NZ:          1253,
	opcode.NUMEQUAL:    1627,
	opcode.NUMNOTEQUAL: 1627,
	opcode.LT:          1662,
	opcode.LE:          1662,
	opcode.GT:          1662,
	opcode.GE:          1662,
	opcode.MIN:         1725,
	opcode.MAX:         1725,
	opcode.WITHIN:      2519,
	opcode.NEWARRAY0:   1774,
	opcode.NEWSTRUCT0:  1774,
	opcode.NEWMAP:      2720,
}

var dynamicCoefficients = [256]func(params *vm.OpcodePriceParams) int64{
	opcode.APPEND:       appendGas,
	opcode.ASSERT:       assertGas,
	opcode.ASSERTMSG:    assertGas,
	opcode.CAT:          catGas,
	opcode.CLEAR:        clearGas,
	opcode.CLEARITEMS:   clearItemsGas,
	opcode.CONVERT:      convertGas,
	opcode.DROP:         dropGas,
	opcode.ENDFINALLY:   endFinallyGas,
	opcode.HASKEY:       hasKeyGas,
	opcode.INITSLOT:     initSlotGas,
	opcode.INITSSLOT:    initSlotGas,
	opcode.ISNULL:       isNullGas,
	opcode.ISTYPE:       isTypeGas,
	opcode.KEYS:         keysGas,
	opcode.LEFT:         substrGas,
	opcode.MEMCPY:       memCpyGas,
	opcode.NEWARRAY:     newArrayGas,
	opcode.NEWARRAYT:    newArrayGas,
	opcode.NEWBUFFER:    newBufferGas,
	opcode.NEWSTRUCT:    newArrayGas,
	opcode.NIP:          dropGas,
	opcode.PACK:         packGas,
	opcode.PACKMAP:      packMapGas,
	opcode.PACKSTRUCT:   packGas,
	opcode.PICKITEM:     pickItemGas,
	opcode.POPITEM:      popItemGas,
	opcode.REMOVE:       removeGas,
	opcode.REVERSEITEMS: reverseItemsGas,
	opcode.REVERSE3:     reverseGas,
	opcode.REVERSE4:     reverseGas,
	opcode.REVERSEN:     reverseGas,
	opcode.RIGHT:        substrGas,
	opcode.ROLL:         rollGas,
	opcode.ROT:          rollGas,
	opcode.SETITEM:      setItemGas,
	opcode.SIZE:         sizeGas,
	opcode.STSFLD0:      stGas,
	opcode.STSFLD1:      stGas,
	opcode.STSFLD2:      stGas,
	opcode.STSFLD3:      stGas,
	opcode.STSFLD4:      stGas,
	opcode.STSFLD5:      stGas,
	opcode.STSFLD6:      stGas,
	opcode.STSFLD:       stGas,
	opcode.STLOC0:       stGas,
	opcode.STLOC1:       stGas,
	opcode.STLOC2:       stGas,
	opcode.STLOC3:       stGas,
	opcode.STLOC4:       stGas,
	opcode.STLOC5:       stGas,
	opcode.STLOC6:       stGas,
	opcode.STLOC:        stGas,
	opcode.STARG0:       stGas,
	opcode.STARG1:       stGas,
	opcode.STARG2:       stGas,
	opcode.STARG3:       stGas,
	opcode.STARG4:       stGas,
	opcode.STARG5:       stGas,
	opcode.STARG6:       stGas,
	opcode.STARG:        stGas,
	opcode.SUBSTR:       substrGas,
	opcode.THROW:        throwGas,
	opcode.UNPACK:       unpackGas,
	opcode.VALUES:       valuesGas,
	opcode.XDROP:        xDropGas,
}

func appendGas(params *vm.OpcodePriceParams) int64 {
	return appendW[0]*int64(params.RefsDelta) + appendW[1]*int64(params.NClonedItems) + appendW[2]
}

func assertGas(params *vm.OpcodePriceParams) int64 {
	return assertW[0]*int64(params.RefsDelta) + assertW[1]*int64(params.Length) + assertW[2]
}

func catGas(params *vm.OpcodePriceParams) int64 {
	return catW[0]*int64(params.Length) + catW[1]
}

func clearGas(params *vm.OpcodePriceParams) int64 {
	return clearW[0]*int64(params.RefsDelta) + clearW[1]*int64(params.Length) + clearW[2]
}

func clearItemsGas(params *vm.OpcodePriceParams) int64 {
	return clearItemsW[0]*int64(params.RefsDelta) + clearItemsW[1]
}

func convertGas(params *vm.OpcodePriceParams) int64 {
	switch params.Typ {
	case stackitem.AnyT:
		return convertAnyW[0]*int64(params.RefsDelta) + convertAnyW[1]
	case stackitem.ArrayT:
		return convertArrOrStructW[0]*int64(params.RefsDelta) + convertArrOrStructW[1]*int64(params.Length) + convertArrOrStructW[2]
	case stackitem.ByteArrayT:
		return convertByteArrOrBufW[0]*int64(params.Length) + convertByteArrOrBufW[1]
	default:
		panic("invalid type")
	}
}

func dropGas(params *vm.OpcodePriceParams) int64 {
	return dropW[0]*int64(params.RefsDelta) + dropW[1]
}

func endFinallyGas(params *vm.OpcodePriceParams) int64 {
	return endFinallyW[0]*int64(params.RefsDelta) + endFinallyW[1]
}

func hasKeyGas(params *vm.OpcodePriceParams) int64 {
	return hasKeyW[0]*int64(params.RefsDelta) + hasKeyW[1]
}

func initSlotGas(params *vm.OpcodePriceParams) int64 {
	return initSlotW[0]*int64(params.RefsDelta) + initSlotW[1]*int64(params.Length) + initSlotW[2]
}

func isNullGas(params *vm.OpcodePriceParams) int64 {
	return isNullW[0]*int64(params.RefsDelta) + isNullW[1]
}

func isTypeGas(params *vm.OpcodePriceParams) int64 {
	return isTypeW[0]*int64(params.RefsDelta) + isTypeW[1]
}

func keysGas(params *vm.OpcodePriceParams) int64 {
	return keysW[0]*int64(params.RefsDelta) + keysW[1]*int64(params.Length) + keysW[2]
}

func memCpyGas(params *vm.OpcodePriceParams) int64 {
	return memCpyW[0]*int64(params.Length) + memCpyW[1]
}

func newArrayGas(params *vm.OpcodePriceParams) int64 {
	if params.Typ == stackitem.ByteArrayT || params.Typ == stackitem.IntegerT {
		return newArrayByteOrIntW[0]*int64(params.Length) + newArrayByteOrIntW[1]
	}
	return newArrayAnyW[0]*int64(params.Length) + newArrayAnyW[1]
}

func newBufferGas(params *vm.OpcodePriceParams) int64 {
	return newBufferW[0]*int64(params.Length) + newBufferW[1]
}

func packGas(params *vm.OpcodePriceParams) int64 {
	return packW[0]*int64(params.Length) + packW[1]
}

func packMapGas(params *vm.OpcodePriceParams) int64 {
	return packMapW[0]*int64(params.RefsDelta) + packMapW[1]*int64(params.Length) + packMapW[2]
}

func pickItemGas(params *vm.OpcodePriceParams) int64 {
	return pickItemW[0]*int64(params.RefsDelta) + pickItemW[1]
}

func popItemGas(params *vm.OpcodePriceParams) int64 {
	return popItemW[0]*int64(params.RefsDelta) + popItemW[1]
}

func removeGas(params *vm.OpcodePriceParams) int64 {
	if params.Typ == stackitem.MapT {
		return removeMapW[0]*int64(params.RefsDelta) + removeMapW[1]*int64(params.Length) + removeMapW[2]
	}
	return removeArrOrStructW[0]*int64(params.RefsDelta) + removeArrOrStructW[1]*int64(params.Length) + removeArrOrStructW[2]
}

func reverseItemsGas(params *vm.OpcodePriceParams) int64 {
	if params.Typ == stackitem.BufferT {
		return reverseItemsBufW[0]*int64(params.Length) + reverseItemsBufW[1]
	}
	return reverseItemsArrW[0]*int64(params.RefsDelta) + reverseItemsArrW[1]*int64(params.Length) + reverseItemsArrW[2]
}

func reverseGas(params *vm.OpcodePriceParams) int64 {
	return reverseW[0]*int64(params.Length) + reverseW[1]
}

func rollGas(params *vm.OpcodePriceParams) int64 {
	return rollW[0]*int64(params.Length) + rollW[1]
}

func setItemGas(params *vm.OpcodePriceParams) int64 {
	return setItemW[0]*int64(params.RefsDelta) + setItemW[1]*int64(params.NClonedItems) + setItemW[2]
}

func sizeGas(params *vm.OpcodePriceParams) int64 {
	return sizeW[0]*int64(params.RefsDelta) + sizeW[1]
}

func stGas(params *vm.OpcodePriceParams) int64 {
	return stW[0]*int64(params.RefsDelta) + stW[1]
}

func substrGas(params *vm.OpcodePriceParams) int64 {
	return substrW[0]*int64(params.Length) + substrW[1]
}

func throwGas(params *vm.OpcodePriceParams) int64 {
	return throwW[0]*int64(params.RefsDelta) + throwW[1]
}

func unpackGas(params *vm.OpcodePriceParams) int64 {
	return unpackW[0]*int64(params.Length) + unpackW[1]
}

func valuesGas(params *vm.OpcodePriceParams) int64 {
	return valuesW[0]*int64(params.RefsDelta) + valuesW[1]*int64(params.Length) + valuesW[2]*int64(params.NClonedItems) + valuesW[3]
}

func xDropGas(params *vm.OpcodePriceParams) int64 {
	return xDropW[0]*int64(params.RefsDelta) + xDropW[1]*int64(params.Length) + xDropW[2]
}
