package opcode

//go:generate stringer -type=Opcode

// Opcode represents a single operation code for the NEO virtual machine.
type Opcode byte

// Viable list of supported instruction constants.
const (
	// Constants
	PUSH0       Opcode = 0x00
	PUSHF       Opcode = PUSH0
	PUSHBYTES1  Opcode = 0x01
	PUSHBYTES2  Opcode = 0x02
	PUSHBYTES3  Opcode = 0x03
	PUSHBYTES4  Opcode = 0x04
	PUSHBYTES5  Opcode = 0x05
	PUSHBYTES6  Opcode = 0x06
	PUSHBYTES7  Opcode = 0x07
	PUSHBYTES8  Opcode = 0x08
	PUSHBYTES9  Opcode = 0x09
	PUSHBYTES10 Opcode = 0x0A
	PUSHBYTES11 Opcode = 0x0B
	PUSHBYTES12 Opcode = 0x0C
	PUSHBYTES13 Opcode = 0x0D
	PUSHBYTES14 Opcode = 0x0E
	PUSHBYTES15 Opcode = 0x0F
	PUSHBYTES16 Opcode = 0x10
	PUSHBYTES17 Opcode = 0x11
	PUSHBYTES18 Opcode = 0x12
	PUSHBYTES19 Opcode = 0x13
	PUSHBYTES20 Opcode = 0x14
	PUSHBYTES21 Opcode = 0x15
	PUSHBYTES22 Opcode = 0x16
	PUSHBYTES23 Opcode = 0x17
	PUSHBYTES24 Opcode = 0x18
	PUSHBYTES25 Opcode = 0x19
	PUSHBYTES26 Opcode = 0x1A
	PUSHBYTES27 Opcode = 0x1B
	PUSHBYTES28 Opcode = 0x1C
	PUSHBYTES29 Opcode = 0x1D
	PUSHBYTES30 Opcode = 0x1E
	PUSHBYTES31 Opcode = 0x1F
	PUSHBYTES32 Opcode = 0x20
	PUSHBYTES33 Opcode = 0x21
	PUSHBYTES34 Opcode = 0x22
	PUSHBYTES35 Opcode = 0x23
	PUSHBYTES36 Opcode = 0x24
	PUSHBYTES37 Opcode = 0x25
	PUSHBYTES38 Opcode = 0x26
	PUSHBYTES39 Opcode = 0x27
	PUSHBYTES40 Opcode = 0x28
	PUSHBYTES41 Opcode = 0x29
	PUSHBYTES42 Opcode = 0x2A
	PUSHBYTES43 Opcode = 0x2B
	PUSHBYTES44 Opcode = 0x2C
	PUSHBYTES45 Opcode = 0x2D
	PUSHBYTES46 Opcode = 0x2E
	PUSHBYTES47 Opcode = 0x2F
	PUSHBYTES48 Opcode = 0x30
	PUSHBYTES49 Opcode = 0x31
	PUSHBYTES50 Opcode = 0x32
	PUSHBYTES51 Opcode = 0x33
	PUSHBYTES52 Opcode = 0x34
	PUSHBYTES53 Opcode = 0x35
	PUSHBYTES54 Opcode = 0x36
	PUSHBYTES55 Opcode = 0x37
	PUSHBYTES56 Opcode = 0x38
	PUSHBYTES57 Opcode = 0x39
	PUSHBYTES58 Opcode = 0x3A
	PUSHBYTES59 Opcode = 0x3B
	PUSHBYTES60 Opcode = 0x3C
	PUSHBYTES61 Opcode = 0x3D
	PUSHBYTES62 Opcode = 0x3E
	PUSHBYTES63 Opcode = 0x3F
	PUSHBYTES64 Opcode = 0x40
	PUSHBYTES65 Opcode = 0x41
	PUSHBYTES66 Opcode = 0x42
	PUSHBYTES67 Opcode = 0x43
	PUSHBYTES68 Opcode = 0x44
	PUSHBYTES69 Opcode = 0x45
	PUSHBYTES70 Opcode = 0x46
	PUSHBYTES71 Opcode = 0x47
	PUSHBYTES72 Opcode = 0x48
	PUSHBYTES73 Opcode = 0x49
	PUSHBYTES74 Opcode = 0x4A
	PUSHBYTES75 Opcode = 0x4B
	PUSHDATA1   Opcode = 0x4C
	PUSHDATA2   Opcode = 0x4D
	PUSHDATA4   Opcode = 0x4E
	PUSHM1      Opcode = 0x4F
	PUSHNULL    Opcode = 0x50
	PUSH1       Opcode = 0x51
	PUSHT       Opcode = PUSH1
	PUSH2       Opcode = 0x52
	PUSH3       Opcode = 0x53
	PUSH4       Opcode = 0x54
	PUSH5       Opcode = 0x55
	PUSH6       Opcode = 0x56
	PUSH7       Opcode = 0x57
	PUSH8       Opcode = 0x58
	PUSH9       Opcode = 0x59
	PUSH10      Opcode = 0x5A
	PUSH11      Opcode = 0x5B
	PUSH12      Opcode = 0x5C
	PUSH13      Opcode = 0x5D
	PUSH14      Opcode = 0x5E
	PUSH15      Opcode = 0x5F
	PUSH16      Opcode = 0x60

	// Flow control
	NOP      Opcode = 0x61
	JMP      Opcode = 0x62
	JMPIF    Opcode = 0x63
	JMPIFNOT Opcode = 0x64
	CALL     Opcode = 0x65
	RET      Opcode = 0x66
	APPCALL  Opcode = 0x67
	SYSCALL  Opcode = 0x68
	TAILCALL Opcode = 0x69

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
	SIZE   Opcode = 0x82

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
	ARRAYSIZE Opcode = 0xC0
	PACK      Opcode = 0xC1
	UNPACK    Opcode = 0xC2
	PICKITEM  Opcode = 0xC3
	SETITEM   Opcode = 0xC4
	NEWARRAY  Opcode = 0xC5
	NEWSTRUCT Opcode = 0xC6
	NEWMAP    Opcode = 0xC7
	APPEND    Opcode = 0xC8
	REVERSE   Opcode = 0xC9
	REMOVE    Opcode = 0xCA
	HASKEY    Opcode = 0xCB
	KEYS      Opcode = 0xCC
	VALUES    Opcode = 0xCD

	// Stack isolation
	CALLI   Opcode = 0xE0
	CALLE   Opcode = 0xE1
	CALLED  Opcode = 0xE2
	CALLET  Opcode = 0xE3
	CALLEDT Opcode = 0xE4

	// Exceptions
	THROW      Opcode = 0xF0
	THROWIFNOT Opcode = 0xF1
)
