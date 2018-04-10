package vm

//go:generate stringer -type=Opcode

// Opcode is an single operational instruction for the GO NEO virtual machine.
type Opcode byte

// List of supported opcodes.
const (
	// Constants
	Opush0       Opcode = 0x00 // An empty array of bytes is pushed onto the stack.
	Opushf       Opcode = Opush0
	Opushbytes1  Opcode = 0x01 // 0x01-0x4B The next opcode bytes is data to be pushed onto the stack
	Opushbytes75 Opcode = 0x4B
	Opushdata1   Opcode = 0x4C // The next byte contains the number of bytes to be pushed onto the stack.
	Opushdata2   Opcode = 0x4D // The next two bytes contain the number of bytes to be pushed onto the stack.
	Opushdata4   Opcode = 0x4E // The next four bytes contain the number of bytes to be pushed onto the stack.
	Opushm1      Opcode = 0x4F // The number -1 is pushed onto the stack.
	Opush1       Opcode = 0x51
	Opusht       Opcode = Opush1
	Opush2       Opcode = 0x52 // The number 2 is pushed onto the stack.
	Opush3       Opcode = 0x53 // The number 3 is pushed onto the stack.
	Opush4       Opcode = 0x54 // The number 4 is pushed onto the stack.
	Opush5       Opcode = 0x55 // The number 5 is pushed onto the stack.
	Opush6       Opcode = 0x56 // The number 6 is pushed onto the stack.
	Opush7       Opcode = 0x57 // The number 7 is pushed onto the stack.
	Opush8       Opcode = 0x58 // The number 8 is pushed onto the stack.
	Opush9       Opcode = 0x59 // The number 9 is pushed onto the stack.
	Opush10      Opcode = 0x5A // The number 10 is pushed onto the stack.
	Opush11      Opcode = 0x5B // The number 11 is pushed onto the stack.
	Opush12      Opcode = 0x5C // The number 12 is pushed onto the stack.
	Opush13      Opcode = 0x5D // The number 13 is pushed onto the stack.
	Opush14      Opcode = 0x5E // The number 14 is pushed onto the stack.
	Opush15      Opcode = 0x5F // The number 15 is pushed onto the stack.
	Opush16      Opcode = 0x60 // The number 16 is pushed onto the stack.

	// Flow control
	Onop      Opcode = 0x61 // No operation.
	Ojmp      Opcode = 0x62
	Ojmpif    Opcode = 0x63
	Ojmpifnot Opcode = 0x64
	Ocall     Opcode = 0x65
	Oret      Opcode = 0x66
	Oappcall  Opcode = 0x67
	Osyscall  Opcode = 0x68
	Otailcall Opcode = 0x69

	// The stack
	Odupfromaltstack Opcode = 0x6A
	Otoaltstack      Opcode = 0x6B // Puts the input onto the top of the alt stack. Removes it from the main stack.
	Ofromaltstack    Opcode = 0x6C // Puts the input onto the top of the main stack. Removes it from the alt stack.
	Oxdrop           Opcode = 0x6D
	Oxswap           Opcode = 0x72
	Oxtuck           Opcode = 0x73
	Odepth           Opcode = 0x74 // Puts the number of stack items onto the stack.
	Odrop            Opcode = 0x75 // Removes the top stack item.
	Odup             Opcode = 0x76 // Duplicates the top stack item.
	Onip             Opcode = 0x77 // Removes the second-to-top stack item.
	Oover            Opcode = 0x78 // Copies the second-to-top stack item to the top.
	Opick            Opcode = 0x79 // The item n back in the stack is copied to the top.
	Oroll            Opcode = 0x7A // The item n back in the stack is moved to the top.
	Orot             Opcode = 0x7B // The top three items on the stack are rotated to the left.
	Oswap            Opcode = 0x7C // The top two items on the stack are swapped.
	Otuck            Opcode = 0x7D // The item at the top of the stack is copied and inserted before the second-to-top item.

	// Splice
	Ocat    Opcode = 0x7E // Concatenates two strings.
	Osubstr Opcode = 0x7F // Returns a section of a string.
	Oleft   Opcode = 0x80 // Keeps only characters left of the specified point in a string.
	Oright  Opcode = 0x81 // Keeps only characters right of the specified point in a string.
	Osize   Opcode = 0x82 // Returns the length of the input string.

	// Bitwise logic
	Oinvert Opcode = 0x83 // Flips all of the bits in the input.
	Oand    Opcode = 0x84 // Boolean and between each bit in the inputs.
	Oor     Opcode = 0x85 // Boolean or between each bit in the inputs.
	Oxor    Opcode = 0x86 // Boolean exclusive or between each bit in the inputs.
	Oequal  Opcode = 0x87 // Returns 1 if the inputs are exactly equal, 0 otherwise.

	// Arithmetic
	// Note: Arithmetic inputs are limited to signed 32-bit integers, but may overflow their output.
	Oinc         Opcode = 0x8B // 1 is added to the input.
	Odec         Opcode = 0x8C // 1 is subtracted from the input.
	Osign        Opcode = 0x8D
	Onegate      Opcode = 0x8F // The sign of the input is flipped.
	Oabs         Opcode = 0x90 // The input is made positive.
	Onot         Opcode = 0x91 // If the input is 0 or 1, it is flipped. Otherwise the output will be 0.
	Onz          Opcode = 0x92 // Returns 0 if the input is 0. 1 otherwise.
	Oadd         Opcode = 0x93 // a is added to b.
	Osub         Opcode = 0x94 // b is subtracted from a.
	Omul         Opcode = 0x95 // a is multiplied by b.
	Odiv         Opcode = 0x96 // a is divided by b.
	Omod         Opcode = 0x97 // Returns the remainder after dividing a by b.
	Oshl         Opcode = 0x98 // Shifts a left b bits, preserving sign.
	Oshr         Opcode = 0x99 // Shifts a right b bits, preserving sign.
	Obooland     Opcode = 0x9A // If both a and b are not 0, the output is 1. Otherwise 0.
	Oboolor      Opcode = 0x9B // If a or b is not 0, the output is 1. Otherwise 0.
	Onumequal    Opcode = 0x9C // Returns 1 if the numbers are equal, 0 otherwise.
	Onumnotequal Opcode = 0x9E // Returns 1 if the numbers are not equal, 0 otherwise.
	Olt          Opcode = 0x9F // Returns 1 if a is less than b, 0 otherwise.
	Ogt          Opcode = 0xA0 // Returns 1 if a is greater than b, 0 otherwise.
	Olte         Opcode = 0xA1 // Returns 1 if a is less than or equal to b, 0 otherwise.
	Ogte         Opcode = 0xA2 // Returns 1 if a is greater than or equal to b, 0 otherwise.
	Omin         Opcode = 0xA3 // Returns the smaller of a and b.
	Omax         Opcode = 0xA4 // Returns the larger of a and b.
	Owithin      Opcode = 0xA5 // Returns 1 if x is within the specified range (left-inclusive), 0 otherwise.

	// Crypto
	Osha1          Opcode = 0xA7 // The input is hashed using SHA-1.
	Osha256        Opcode = 0xA8 // The input is hashed using SHA-256.
	Ohash160       Opcode = 0xA9
	Ohash256       Opcode = 0xAA
	Ochecksig      Opcode = 0xAC
	Ocheckmultisig Opcode = 0xAE

	// array
	Oarraysize Opcode = 0xC0
	Opack      Opcode = 0xC1
	Ounpack    Opcode = 0xC2
	Opickitem  Opcode = 0xC3
	Osetitem   Opcode = 0xC4
	Onewarray  Opcode = 0xC5 // Pops size from stack and creates a new array with that size, and pushes the array into the stack
	Onewstruct Opcode = 0xC6
	Oappend    Opcode = 0xC8
	Oreverse   Opcode = 0xC9
	Oremove    Opcode = 0xCA

	// exceptions
	Othrow      Opcode = 0xF0
	Othrowifnot Opcode = 0xF1
)
