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
	catW                 = []int64{8, 2706}
	clearW               = []int64{93, -15, 1515}
	clearItemsW          = []int64{103, 1455}
	convertArrOrStructW  = []int64{318, 3435}
	convertByteArrOrBufW = []int64{7, 3417}
	dropW                = []int64{99, 1486}
	dupW                 = []int64{7, 3142}
	hasKeyW              = []int64{99, 2575}
	initSlotW            = []int64{155, 2156}
	isNullW              = []int64{97, 1236}
	isTypeW              = []int64{97, 1154}
	keysW                = []int64{1419, 2606}
	memCpyW              = []int64{1, 3514}
	newArrayAnyW         = []int64{126, 2718}
	newArrayByteOrIntW   = []int64{849, 2718}
	newBufferW           = []int64{6, 2507}
	packW                = []int64{173, 2649}
	packMapW             = []int64{80, 5281, 3481}
	pickItemW            = []int64{91, 6, 2751}
	popItemW             = []int64{91, 3078}
	removeArrW           = []int64{98, 9, 1991}
	removeMapW           = []int64{96, 706, 6776}
	reverseItemsArrW     = []int64{98, 19, 2043}
	reverseItemsBufW     = []int64{9, 1690}
	reverseW             = []int64{19, 1702}
	rollW                = []int64{5, 1910}
	setItemW             = []int64{99, 350, 2942}
	sizeW                = []int64{100, 2693}
	stW                  = []int64{98, 1599}
	substrW              = []int64{7, 2908}
	unpackW              = []int64{254, 2604}
	valuesW              = []int64{307, 369, 9868}
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

var (
	coefficients          = scaleCoefficients(notScaledCoefficients, factor)
	notScaledCoefficients = [256]int64{
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
)

// OpcodeV1 returns opcode pricing in the unit of Datoshi*[vm.OpcodePriceMultiplier]
// used since Gorgon hardfork, supporting dynamic (parameter-dependent) costs.
func OpcodeV1(base int64, op opcode.Opcode, args *vm.OpcodePriceArgs) int64 {
	if args != nil {
		return opcodeDynamic(base, op, args)
	}
	return opcodeStatic(base, op)
}

// OpcodeStatic returns static opcode prices used since Gorgon hardfork.
func opcodeStatic(base int64, op opcode.Opcode) int64 {
	return base * staticCoefficients[op]
}

var (
	staticCoefficients          = scaleCoefficients(notScaledStaticCoefficients, factor)
	notScaledStaticCoefficients = [256]int64{
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
)

func scaleCoefficients(src [256]int64, mul int64) [256]int64 {
	var dst [256]int64
	for i, c := range src {
		dst[i] = c * mul
	}
	return dst
}

// OpcodeDynamic returns dynamic opcode prices used since Gorgon hardfork.
func opcodeDynamic(base int64, op opcode.Opcode, args *vm.OpcodePriceArgs) int64 {
	return base * max(1, dynamicCoefficients[op](args))
}

var dynamicCoefficients = [256]func(args *vm.OpcodePriceArgs) int64{
	opcode.APPEND:       appendGas,
	opcode.CAT:          catGas,
	opcode.CLEAR:        clearGas,
	opcode.CLEARITEMS:   clearItemsGas,
	opcode.CONVERT:      convertGas,
	opcode.DROP:         dropGas,
	opcode.DUP:          dupGas,
	opcode.HASKEY:       hasKeyGas,
	opcode.INITSLOT:     initSlotGas,
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
	opcode.OVER:         dupGas,
	opcode.PACK:         packGas,
	opcode.PACKMAP:      packMapGas,
	opcode.PACKSTRUCT:   packGas,
	opcode.PICK:         dupGas,
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
	opcode.UNPACK:       unpackGas,
	opcode.VALUES:       valuesGas,
	opcode.XDROP:        xDropGas,
}

func appendGas(args *vm.OpcodePriceArgs) int64 {
	return appendW[0]*int64(args.RefsDelta) + appendW[1]*int64(args.NClonedItems) + appendW[2]
}

func catGas(args *vm.OpcodePriceArgs) int64 {
	return catW[0]*int64(args.Length) + catW[1]
}

func clearGas(args *vm.OpcodePriceArgs) int64 {
	return clearW[0]*int64(args.RefsDelta) + clearW[1]*int64(args.Length) + clearW[2]
}

func clearItemsGas(args *vm.OpcodePriceArgs) int64 {
	return clearItemsW[0]*int64(args.RefsDelta) + clearItemsW[1]
}

func convertGas(args *vm.OpcodePriceArgs) int64 {
	if args.Typ == stackitem.ArrayT {
		return convertArrOrStructW[0]*int64(args.Length) + convertArrOrStructW[1]
	}
	return convertByteArrOrBufW[0]*int64(args.Length) + convertByteArrOrBufW[1]
}

func dropGas(args *vm.OpcodePriceArgs) int64 {
	return dropW[0]*int64(args.RefsDelta) + dropW[1]
}

func dupGas(args *vm.OpcodePriceArgs) int64 {
	return dupW[0]*int64(args.Length) + dupW[1]
}

func hasKeyGas(args *vm.OpcodePriceArgs) int64 {
	return hasKeyW[0]*int64(args.RefsDelta) + hasKeyW[1]
}

func initSlotGas(args *vm.OpcodePriceArgs) int64 {
	return initSlotW[0]*int64(args.Length) + initSlotW[1]
}

func isNullGas(args *vm.OpcodePriceArgs) int64 {
	return isNullW[0]*int64(args.RefsDelta) + isNullW[1]
}

func isTypeGas(args *vm.OpcodePriceArgs) int64 {
	return isTypeW[0]*int64(args.RefsDelta) + isTypeW[1]
}

func keysGas(args *vm.OpcodePriceArgs) int64 {
	return keysW[0]*int64(args.Length) + keysW[1]
}

func memCpyGas(args *vm.OpcodePriceArgs) int64 {
	return memCpyW[0]*int64(args.Length) + memCpyW[1]
}

func newArrayGas(args *vm.OpcodePriceArgs) int64 {
	if args.Typ == stackitem.ByteArrayT || args.Typ == stackitem.IntegerT {
		return newArrayByteOrIntW[0]*int64(args.Length) + newArrayByteOrIntW[1]
	}
	return newArrayAnyW[0]*int64(args.Length) + newArrayAnyW[1]
}

func newBufferGas(args *vm.OpcodePriceArgs) int64 {
	return newBufferW[0]*int64(args.Length) + newBufferW[1]
}

func packGas(args *vm.OpcodePriceArgs) int64 {
	return packW[0]*int64(args.Length) + packW[1]
}

func packMapGas(args *vm.OpcodePriceArgs) int64 {
	return packMapW[0]*int64(args.RefsDelta) + packMapW[1]*int64(args.Length) + packMapW[2]
}

func pickItemGas(args *vm.OpcodePriceArgs) int64 {
	return pickItemW[0]*int64(args.RefsDelta) + pickItemW[1]*int64(args.Length) + pickItemW[2]
}

func popItemGas(args *vm.OpcodePriceArgs) int64 {
	return popItemW[0]*int64(args.RefsDelta) + popItemW[1]
}

func removeGas(args *vm.OpcodePriceArgs) int64 {
	if args.Typ == stackitem.MapT {
		return removeMapW[0]*int64(args.RefsDelta) + removeMapW[1]*int64(args.Length) + removeMapW[2]
	}
	return removeArrW[0]*int64(args.RefsDelta) + removeArrW[1]*int64(args.Length) + removeArrW[2]
}

func reverseItemsGas(args *vm.OpcodePriceArgs) int64 {
	if args.Typ == stackitem.BufferT {
		return reverseItemsBufW[0]*int64(args.Length) + reverseItemsBufW[1]
	}
	return reverseItemsArrW[0]*int64(args.RefsDelta) + reverseItemsArrW[1]*int64(args.Length) + reverseItemsArrW[2]
}

func reverseGas(args *vm.OpcodePriceArgs) int64 {
	return reverseW[0]*int64(args.Length) + reverseW[1]
}

func rollGas(args *vm.OpcodePriceArgs) int64 {
	return rollW[0]*int64(args.Length) + rollW[1]
}

func setItemGas(args *vm.OpcodePriceArgs) int64 {
	return setItemW[0]*int64(args.RefsDelta) + setItemW[1]*int64(args.NClonedItems) + setItemW[2]
}

func sizeGas(args *vm.OpcodePriceArgs) int64 {
	return sizeW[0]*int64(args.RefsDelta) + sizeW[1]
}

func stGas(args *vm.OpcodePriceArgs) int64 {
	return stW[0]*int64(args.RefsDelta) + stW[1]
}

func substrGas(args *vm.OpcodePriceArgs) int64 {
	return substrW[0]*int64(args.Length) + substrW[1]
}

func unpackGas(args *vm.OpcodePriceArgs) int64 {
	return unpackW[0]*int64(args.Length) + unpackW[1]
}

func valuesGas(args *vm.OpcodePriceArgs) int64 {
	return valuesW[0]*int64(args.Length) + valuesW[1]*int64(args.NClonedItems) + valuesW[2]
}

func xDropGas(args *vm.OpcodePriceArgs) int64 {
	return xDropW[0]*int64(args.RefsDelta) + xDropW[1]*int64(args.Length) + xDropW[2]
}
