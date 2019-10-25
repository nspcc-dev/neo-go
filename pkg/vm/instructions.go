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
	PUSHBYTES2  Instruction = 0x02
	PUSHBYTES3  Instruction = 0x03
	PUSHBYTES4  Instruction = 0x04
	PUSHBYTES5  Instruction = 0x05
	PUSHBYTES6  Instruction = 0x06
	PUSHBYTES7  Instruction = 0x07
	PUSHBYTES8  Instruction = 0x08
	PUSHBYTES9  Instruction = 0x09
	PUSHBYTES10 Instruction = 0x0A
	PUSHBYTES11 Instruction = 0x0B
	PUSHBYTES12 Instruction = 0x0C
	PUSHBYTES13 Instruction = 0x0D
	PUSHBYTES14 Instruction = 0x0E
	PUSHBYTES15 Instruction = 0x0F
	PUSHBYTES16 Instruction = 0x10
	PUSHBYTES17 Instruction = 0x11
	PUSHBYTES18 Instruction = 0x12
	PUSHBYTES19 Instruction = 0x13
	PUSHBYTES20 Instruction = 0x14
	PUSHBYTES21 Instruction = 0x15
	PUSHBYTES22 Instruction = 0x16
	PUSHBYTES23 Instruction = 0x17
	PUSHBYTES24 Instruction = 0x18
	PUSHBYTES25 Instruction = 0x19
	PUSHBYTES26 Instruction = 0x1A
	PUSHBYTES27 Instruction = 0x1B
	PUSHBYTES28 Instruction = 0x1C
	PUSHBYTES29 Instruction = 0x1D
	PUSHBYTES30 Instruction = 0x1E
	PUSHBYTES31 Instruction = 0x1F
	PUSHBYTES32 Instruction = 0x20
	PUSHBYTES33 Instruction = 0x21
	PUSHBYTES34 Instruction = 0x22
	PUSHBYTES35 Instruction = 0x23
	PUSHBYTES36 Instruction = 0x24
	PUSHBYTES37 Instruction = 0x25
	PUSHBYTES38 Instruction = 0x26
	PUSHBYTES39 Instruction = 0x27
	PUSHBYTES40 Instruction = 0x28
	PUSHBYTES41 Instruction = 0x29
	PUSHBYTES42 Instruction = 0x2A
	PUSHBYTES43 Instruction = 0x2B
	PUSHBYTES44 Instruction = 0x2C
	PUSHBYTES45 Instruction = 0x2D
	PUSHBYTES46 Instruction = 0x2E
	PUSHBYTES47 Instruction = 0x2F
	PUSHBYTES48 Instruction = 0x30
	PUSHBYTES49 Instruction = 0x31
	PUSHBYTES50 Instruction = 0x32
	PUSHBYTES51 Instruction = 0x33
	PUSHBYTES52 Instruction = 0x34
	PUSHBYTES53 Instruction = 0x35
	PUSHBYTES54 Instruction = 0x36
	PUSHBYTES55 Instruction = 0x37
	PUSHBYTES56 Instruction = 0x38
	PUSHBYTES57 Instruction = 0x39
	PUSHBYTES58 Instruction = 0x3A
	PUSHBYTES59 Instruction = 0x3B
	PUSHBYTES60 Instruction = 0x3C
	PUSHBYTES61 Instruction = 0x3D
	PUSHBYTES62 Instruction = 0x3E
	PUSHBYTES63 Instruction = 0x3F
	PUSHBYTES64 Instruction = 0x40
	PUSHBYTES65 Instruction = 0x41
	PUSHBYTES66 Instruction = 0x42
	PUSHBYTES67 Instruction = 0x43
	PUSHBYTES68 Instruction = 0x44
	PUSHBYTES69 Instruction = 0x45
	PUSHBYTES70 Instruction = 0x46
	PUSHBYTES71 Instruction = 0x47
	PUSHBYTES72 Instruction = 0x48
	PUSHBYTES73 Instruction = 0x49
	PUSHBYTES74 Instruction = 0x4A
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
	VERIFY        Instruction = 0xAD
	CHECKMULTISIG Instruction = 0xAE

	// Advanced data structures (arrays, structures, maps)
	ARRAYSIZE Instruction = 0xC0
	PACK      Instruction = 0xC1
	UNPACK    Instruction = 0xC2
	PICKITEM  Instruction = 0xC3
	SETITEM   Instruction = 0xC4
	NEWARRAY  Instruction = 0xC5
	NEWSTRUCT Instruction = 0xC6
	NEWMAP    Instruction = 0xC7
	APPEND    Instruction = 0xC8
	REVERSE   Instruction = 0xC9
	REMOVE    Instruction = 0xCA
	HASKEY    Instruction = 0xCB
	KEYS      Instruction = 0xCC
	VALUES    Instruction = 0xCD

	// Stack isolation
	CALLI   Instruction = 0xE0
	CALLE   Instruction = 0xE1
	CALLED  Instruction = 0xE2
	CALLET  Instruction = 0xE3
	CALLEDT Instruction = 0xE4

	// Exceptions
	THROW      Instruction = 0xF0
	THROWIFNOT Instruction = 0xF1
)
