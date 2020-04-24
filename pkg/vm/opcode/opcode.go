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

	// Legacy
	OLDPUSH1 Opcode = 0x51

	RET      Opcode = 0x66
	APPCALL  Opcode = 0x67
	SYSCALL  Opcode = 0x68
	TAILCALL Opcode = 0x69

	ISNULL Opcode = 0x70

	// Stack
	DUPFROMALTSTACK Opcode = 0x6A
	TOALTSTACK      Opcode = 0x6B
	FROMALTSTACK    Opcode = 0x6C
	XDROP           Opcode = 0x6D
	XSWAP           Opcode = 0x72
	XTUCK           Opcode = 0x73
	DEPTH           Opcode = 0x74
	DROP            Opcode = 0x75
	DUP             Opcode = 0x76
	NIP             Opcode = 0x77
	OVER            Opcode = 0x78
	PICK            Opcode = 0x79
	ROLL            Opcode = 0x7A
	ROT             Opcode = 0x7B
	SWAP            Opcode = 0x7C
	TUCK            Opcode = 0x7D

	// Splice
	CAT    Opcode = 0x7E
	SUBSTR Opcode = 0x7F
	LEFT   Opcode = 0x80
	RIGHT  Opcode = 0x81

	// Bitwise logic
	INVERT Opcode = 0x83
	AND    Opcode = 0x84
	OR     Opcode = 0x85
	XOR    Opcode = 0x86
	EQUAL  Opcode = 0x87

	// Arithmetic
	INC         Opcode = 0x8B
	DEC         Opcode = 0x8C
	SIGN        Opcode = 0x8D
	NEGATE      Opcode = 0x8F
	ABS         Opcode = 0x90
	NOT         Opcode = 0x91
	NZ          Opcode = 0x92
	ADD         Opcode = 0x93
	SUB         Opcode = 0x94
	MUL         Opcode = 0x95
	DIV         Opcode = 0x96
	MOD         Opcode = 0x97
	SHL         Opcode = 0x98
	SHR         Opcode = 0x99
	BOOLAND     Opcode = 0x9A
	BOOLOR      Opcode = 0x9B
	NUMEQUAL    Opcode = 0x9C
	NUMNOTEQUAL Opcode = 0x9E
	LT          Opcode = 0x9F
	GT          Opcode = 0xA0
	LTE         Opcode = 0xA1
	GTE         Opcode = 0xA2
	MIN         Opcode = 0xA3
	MAX         Opcode = 0xA4
	WITHIN      Opcode = 0xA5

	// Crypto
	SHA1          Opcode = 0xA7
	SHA256        Opcode = 0xA8
	HASH160       Opcode = 0xA9
	HASH256       Opcode = 0xAA
	CHECKSIG      Opcode = 0xAC
	VERIFY        Opcode = 0xAD
	CHECKMULTISIG Opcode = 0xAE

	// Advanced data structures (arrays, structures, maps)
	PACK      Opcode = 0xC0
	UNPACK    Opcode = 0xC1
	NEWARRAY0 Opcode = 0xC2
	NEWARRAY  Opcode = 0xC3
	// NEWARRAYT   Opcode = 0xC4
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
	ISTYPE Opcode = 0xD9

	// Exceptions
	THROW      Opcode = 0xF0
	THROWIFNOT Opcode = 0xF1
)
