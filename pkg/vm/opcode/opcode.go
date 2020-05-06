package opcode

//go:generate stringer -type=Opcode

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
	JMPL      Opcode = 0x23
	JMPIF     Opcode = 0x24
	JMPIFL    Opcode = 0x25
	JMPIFNOT  Opcode = 0x26
	JMPIFNOTL Opcode = 0x27
	JMPEQ     Opcode = 0x28
	JMPEQL    Opcode = 0x29
	JMPNE     Opcode = 0x2A
	JMPNEL    Opcode = 0x2B
	JMPGT     Opcode = 0x2C
	JMPGTL    Opcode = 0x2D
	JMPGE     Opcode = 0x2E
	JMPGEL    Opcode = 0x2F
	JMPLT     Opcode = 0x30
	JMPLTL    Opcode = 0x31
	JMPLE     Opcode = 0x32
	JMPLEL    Opcode = 0x33
	CALL      Opcode = 0x34
	CALLL     Opcode = 0x35

	// Stack
	DEPTH Opcode = 0x43
	DROP  Opcode = 0x45
	NIP   Opcode = 0x46
	XDROP Opcode = 0x48
	// CLEAR    Opcode = 0x49
	DUP      Opcode = 0x4A
	OVER     Opcode = 0x4B
	PICK     Opcode = 0x4D
	TUCK     Opcode = 0x4E
	SWAP     Opcode = 0x50
	OLDPUSH1 Opcode = 0x51 // FIXME remove #927
	ROT      Opcode = 0x51
	ROLL     Opcode = 0x52
	// REVERSE3 Opcode = 0x53
	// REVERSE4 Opcode = 0x54
	// REVERSEN Opcode = 0x55

	RET      Opcode = 0x66
	APPCALL  Opcode = 0x67
	SYSCALL  Opcode = 0x68
	TAILCALL Opcode = 0x69

	// Old stack opcodes
	DUPFROMALTSTACK Opcode = 0x6A
	TOALTSTACK      Opcode = 0x6B
	FROMALTSTACK    Opcode = 0x6C
	XSWAP           Opcode = 0x72
	XTUCK           Opcode = 0x73

	// Splice
	CAT    Opcode = 0x7E
	SUBSTR Opcode = 0x7F
	LEFT   Opcode = 0x80
	RIGHT  Opcode = 0x81

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
	NEWARRAYT    Opcode = 0xC4
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

	// Types
	ISNULL  Opcode = 0xD8
	ISTYPE  Opcode = 0xD9
	CONVERT Opcode = 0xDB

	// Exceptions
	THROW      Opcode = 0xF0
	THROWIFNOT Opcode = 0xF1
)
