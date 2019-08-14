package vm

//go:generate stringer -type=Instruction

// Instruction represents an single operation for the NEO virtual machine.
type Instruction byte

// Viable list of supported instruction constants.
const (
	// Constants
	PUSH0       Instruction = 0x00
	PUSHF       Instruction = PUSH0
	PUSHBYTES1  Instruction = 0x01
	PUSHBYTES75 Instruction = 0x4B
	PUSHDATA1   Instruction = 0x4C
	PUSHDATA2   Instruction = 0x4D
	PUSHDATA4   Instruction = 0x4E
	PUSHM1      Instruction = 0x4F
	PUSH1       Instruction = 0x51
	PUSHT       Instruction = PUSH1
	PUSH2       Instruction = 0x52
	PUSH3       Instruction = 0x53
	PUSH4       Instruction = 0x54
	PUSH5       Instruction = 0x55
	PUSH6       Instruction = 0x56
	PUSH7       Instruction = 0x57
	PUSH8       Instruction = 0x58
	PUSH9       Instruction = 0x59
	PUSH10      Instruction = 0x5A
	PUSH11      Instruction = 0x5B
	PUSH12      Instruction = 0x5C
	PUSH13      Instruction = 0x5D
	PUSH14      Instruction = 0x5E
	PUSH15      Instruction = 0x5F
	PUSH16      Instruction = 0x60

	// Flow control
	NOP      Instruction = 0x61
	JMP      Instruction = 0x62
	JMPIF    Instruction = 0x63
	JMPIFNOT Instruction = 0x64
	CALL     Instruction = 0x65
	RET      Instruction = 0x66
	APPCALL  Instruction = 0x67
	SYSCALL  Instruction = 0x68
	TAILCALL Instruction = 0x69

	// Stack
	DUPFROMALTSTACK Instruction = 0x6A
	TOALTSTACK      Instruction = 0x6B
	FROMALTSTACK    Instruction = 0x6C
	XDROP           Instruction = 0x6D
	XSWAP           Instruction = 0x72
	XTUCK           Instruction = 0x73
	DEPTH           Instruction = 0x74
	DROP            Instruction = 0x75
	DUP             Instruction = 0x76
	NIP             Instruction = 0x77
	OVER            Instruction = 0x78
	PICK            Instruction = 0x79
	ROLL            Instruction = 0x7A
	ROT             Instruction = 0x7B
	SWAP            Instruction = 0x7C
	TUCK            Instruction = 0x7D

	// Splice
	CAT    Instruction = 0x7E
	SUBSTR Instruction = 0x7F
	LEFT   Instruction = 0x80
	RIGHT  Instruction = 0x81
	SIZE   Instruction = 0x82

	// Bitwise logic
	INVERT Instruction = 0x83
	AND    Instruction = 0x84
	OR     Instruction = 0x85
	XOR    Instruction = 0x86
	EQUAL  Instruction = 0x87

	// Arithmetic
	INC         Instruction = 0x8B
	DEC         Instruction = 0x8C
	SIGN        Instruction = 0x8D
	NEGATE      Instruction = 0x8F
	ABS         Instruction = 0x90
	NOT         Instruction = 0x91
	NZ          Instruction = 0x92
	ADD         Instruction = 0x93
	SUB         Instruction = 0x94
	MUL         Instruction = 0x95
	DIV         Instruction = 0x96
	MOD         Instruction = 0x97
	SHL         Instruction = 0x98
	SHR         Instruction = 0x99
	BOOLAND     Instruction = 0x9A
	BOOLOR      Instruction = 0x9B
	NUMEQUAL    Instruction = 0x9C
	NUMNOTEQUAL Instruction = 0x9E
	LT          Instruction = 0x9F
	GT          Instruction = 0xA0
	LTE         Instruction = 0xA1
	GTE         Instruction = 0xA2
	MIN         Instruction = 0xA3
	MAX         Instruction = 0xA4
	WITHIN      Instruction = 0xA5

	// Crypto
	SHA1          Instruction = 0xA7
	SHA256        Instruction = 0xA8
	HASH160       Instruction = 0xA9
	HASH256       Instruction = 0xAA
	CHECKSIG      Instruction = 0xAC
	CHECKMULTISIG Instruction = 0xAE

	// Array
	ARRAYSIZE Instruction = 0xC0
	PACK      Instruction = 0xC1
	UNPACK    Instruction = 0xC2
	PICKITEM  Instruction = 0xC3
	SETITEM   Instruction = 0xC4
	NEWARRAY  Instruction = 0xC5
	NEWSTRUCT Instruction = 0xC6
	APPEND    Instruction = 0xC8
	REVERSE   Instruction = 0xC9
	REMOVE    Instruction = 0xCA

	// Exceptions
	THROW      Instruction = 0xF0
	THROWIFNOT Instruction = 0xF1
)
