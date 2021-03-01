package opcode

//go:generate stringer -type=Opcode -linecomment

// Opcode represents a single operation code for the NEO virtual machine.
type Opcode byte

// Viable list of supported instruction constants.
const (
	// Constants
	PUSHINT8   Opcode = 0x00
	PUSHINT16  Opcode = 0x01
	PUSHINT32  Opcode = 0x02
	PUSHINT64  Opcode = 0x03
	PUSHINT128 Opcode = 0x04
	PUSHINT256 Opcode = 0x05

	PUSHA    Opcode = 0x0A
	PUSHNULL Opcode = 0x0B

	PUSHDATA1 Opcode = 0x0C
	PUSHDATA2 Opcode = 0x0D
	PUSHDATA4 Opcode = 0x0E

	PUSHM1 Opcode = 0x0F
	PUSH0  Opcode = 0x10
	PUSHF  Opcode = PUSH0
	PUSH1  Opcode = 0x11
	PUSHT  Opcode = PUSH1
	PUSH2  Opcode = 0x12
	PUSH3  Opcode = 0x13
	PUSH4  Opcode = 0x14
	PUSH5  Opcode = 0x15
	PUSH6  Opcode = 0x16
	PUSH7  Opcode = 0x17
	PUSH8  Opcode = 0x18
	PUSH9  Opcode = 0x19
	PUSH10 Opcode = 0x1A
	PUSH11 Opcode = 0x1B
	PUSH12 Opcode = 0x1C
	PUSH13 Opcode = 0x1D
	PUSH14 Opcode = 0x1E
	PUSH15 Opcode = 0x1F
	PUSH16 Opcode = 0x20

	// Flow control
	NOP       Opcode = 0x21
	JMP       Opcode = 0x22
	JMPL      Opcode = 0x23 // JMP_L
	JMPIF     Opcode = 0x24
	JMPIFL    Opcode = 0x25 // JMPIF_L
	JMPIFNOT  Opcode = 0x26
	JMPIFNOTL Opcode = 0x27 // JMPIFNOT_L
	JMPEQ     Opcode = 0x28
	JMPEQL    Opcode = 0x29 // JMPEQ_L
	JMPNE     Opcode = 0x2A
	JMPNEL    Opcode = 0x2B // JMPNE_L
	JMPGT     Opcode = 0x2C
	JMPGTL    Opcode = 0x2D // JMPGT_L
	JMPGE     Opcode = 0x2E
	JMPGEL    Opcode = 0x2F // JMPGE_L
	JMPLT     Opcode = 0x30
	JMPLTL    Opcode = 0x31 // JMPLT_L
	JMPLE     Opcode = 0x32
	JMPLEL    Opcode = 0x33 // JMPLE_L
	CALL      Opcode = 0x34
	CALLL     Opcode = 0x35 // CALL_L
	CALLA     Opcode = 0x36
	CALLT     Opcode = 0x37

	// Exceptions
	ABORT      Opcode = 0x38
	ASSERT     Opcode = 0x39
	THROW      Opcode = 0x3A
	TRY        Opcode = 0x3B
	TRYL       Opcode = 0x3C // TRY_L
	ENDTRY     Opcode = 0x3D
	ENDTRYL    Opcode = 0x3E // ENDTRY_L
	ENDFINALLY Opcode = 0x3F

	RET     Opcode = 0x40
	SYSCALL Opcode = 0x41

	// Stack
	DEPTH    Opcode = 0x43
	DROP     Opcode = 0x45
	NIP      Opcode = 0x46
	XDROP    Opcode = 0x48
	CLEAR    Opcode = 0x49
	DUP      Opcode = 0x4A
	OVER     Opcode = 0x4B
	PICK     Opcode = 0x4D
	TUCK     Opcode = 0x4E
	SWAP     Opcode = 0x50
	ROT      Opcode = 0x51
	ROLL     Opcode = 0x52
	REVERSE3 Opcode = 0x53
	REVERSE4 Opcode = 0x54
	REVERSEN Opcode = 0x55

	// Slots
	INITSSLOT Opcode = 0x56
	INITSLOT  Opcode = 0x57
	LDSFLD0   Opcode = 0x58
	LDSFLD1   Opcode = 0x59
	LDSFLD2   Opcode = 0x5A
	LDSFLD3   Opcode = 0x5B
	LDSFLD4   Opcode = 0x5C
	LDSFLD5   Opcode = 0x5D
	LDSFLD6   Opcode = 0x5E
	LDSFLD    Opcode = 0x5F
	STSFLD0   Opcode = 0x60
	STSFLD1   Opcode = 0x61
	STSFLD2   Opcode = 0x62
	STSFLD3   Opcode = 0x63
	STSFLD4   Opcode = 0x64
	STSFLD5   Opcode = 0x65
	STSFLD6   Opcode = 0x66
	STSFLD    Opcode = 0x67
	LDLOC0    Opcode = 0x68
	LDLOC1    Opcode = 0x69
	LDLOC2    Opcode = 0x6A
	LDLOC3    Opcode = 0x6B
	LDLOC4    Opcode = 0x6C
	LDLOC5    Opcode = 0x6D
	LDLOC6    Opcode = 0x6E
	LDLOC     Opcode = 0x6F
	STLOC0    Opcode = 0x70
	STLOC1    Opcode = 0x71
	STLOC2    Opcode = 0x72
	STLOC3    Opcode = 0x73
	STLOC4    Opcode = 0x74
	STLOC5    Opcode = 0x75
	STLOC6    Opcode = 0x76
	STLOC     Opcode = 0x77
	LDARG0    Opcode = 0x78
	LDARG1    Opcode = 0x79
	LDARG2    Opcode = 0x7A
	LDARG3    Opcode = 0x7B
	LDARG4    Opcode = 0x7C
	LDARG5    Opcode = 0x7D
	LDARG6    Opcode = 0x7E
	LDARG     Opcode = 0x7F
	STARG0    Opcode = 0x80
	STARG1    Opcode = 0x81
	STARG2    Opcode = 0x82
	STARG3    Opcode = 0x83
	STARG4    Opcode = 0x84
	STARG5    Opcode = 0x85
	STARG6    Opcode = 0x86
	STARG     Opcode = 0x87

	// Splice
	NEWBUFFER Opcode = 0x88
	MEMCPY    Opcode = 0x89
	CAT       Opcode = 0x8B
	SUBSTR    Opcode = 0x8C
	LEFT      Opcode = 0x8D
	RIGHT     Opcode = 0x8E

	// Bitwise logic
	INVERT   Opcode = 0x90
	AND      Opcode = 0x91
	OR       Opcode = 0x92
	XOR      Opcode = 0x93
	EQUAL    Opcode = 0x97
	NOTEQUAL Opcode = 0x98

	// Arithmetic
	SIGN        Opcode = 0x99
	ABS         Opcode = 0x9A
	NEGATE      Opcode = 0x9B
	INC         Opcode = 0x9C
	DEC         Opcode = 0x9D
	ADD         Opcode = 0x9E
	SUB         Opcode = 0x9F
	MUL         Opcode = 0xA0
	DIV         Opcode = 0xA1
	MOD         Opcode = 0xA2
	POW         Opcode = 0xA3
	SHL         Opcode = 0xA8
	SHR         Opcode = 0xA9
	NOT         Opcode = 0xAA
	BOOLAND     Opcode = 0xAB
	BOOLOR      Opcode = 0xAC
	NZ          Opcode = 0xB1
	NUMEQUAL    Opcode = 0xB3
	NUMNOTEQUAL Opcode = 0xB4
	LT          Opcode = 0xB5
	LTE         Opcode = 0xB6
	GT          Opcode = 0xB7
	GTE         Opcode = 0xB8
	MIN         Opcode = 0xB9
	MAX         Opcode = 0xBA
	WITHIN      Opcode = 0xBB

	// Advanced data structures (arrays, structures, maps)
	PACK         Opcode = 0xC0
	UNPACK       Opcode = 0xC1
	NEWARRAY0    Opcode = 0xC2
	NEWARRAY     Opcode = 0xC3
	NEWARRAYT    Opcode = 0xC4 // NEWARRAY_T
	NEWSTRUCT0   Opcode = 0xC5
	NEWSTRUCT    Opcode = 0xC6
	NEWMAP       Opcode = 0xC8
	SIZE         Opcode = 0xCA
	HASKEY       Opcode = 0xCB
	KEYS         Opcode = 0xCC
	VALUES       Opcode = 0xCD
	PICKITEM     Opcode = 0xCE
	APPEND       Opcode = 0xCF
	SETITEM      Opcode = 0xD0
	REVERSEITEMS Opcode = 0xD1
	REMOVE       Opcode = 0xD2
	CLEARITEMS   Opcode = 0xD3
	POPITEM      Opcode = 0xD4

	// Types
	ISNULL  Opcode = 0xD8
	ISTYPE  Opcode = 0xD9
	CONVERT Opcode = 0xDB
)

// IsValid returns true if the opcode passed is valid (defined in the VM).
func IsValid(op Opcode) bool {
	_, ok := _Opcode_map[op] // We rely on stringer here, it has a map anyway.
	return ok
}
