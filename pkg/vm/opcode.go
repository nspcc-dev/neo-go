package vm

// OpCode is an single operational instruction for the GO NEO virtual machine.
type OpCode uint8

// List of supported opcodes.
const (
	// Constants
	OpPush0       OpCode = 0x00 // An empty array of bytes is pushed onto the stack.
	OpPushF       OpCode = OpPush0
	OpPushBytes1  OpCode = 0x01 // 0x01-0x4B The next opcode bytes is data to be pushed onto the stack
	OpPushBytes75 OpCode = 0x4B
	OpPushData1   OpCode = 0x4C // The next byte contains the number of bytes to be pushed onto the stack.
	OpPushData2   OpCode = 0x4D // The next two bytes contain the number of bytes to be pushed onto the stack.
	OpPushData4   OpCode = 0x4E // The next four bytes contain the number of bytes to be pushed onto the stack.
	OpPushM1      OpCode = 0x4F // The number -1 is pushed onto the stack.
	OpPush1       OpCode = 0x51
	OpPushT       OpCode = OpPush1
	OpPush2       OpCode = 0x52 // The number 2 is pushed onto the stack.
	OpPush3       OpCode = 0x53 // The number 3 is pushed onto the stack.
	OpPush4       OpCode = 0x54 // The number 4 is pushed onto the stack.
	OpPush5       OpCode = 0x55 // The number 5 is pushed onto the stack.
	OpPush6       OpCode = 0x56 // The number 6 is pushed onto the stack.
	OpPush7       OpCode = 0x57 // The number 7 is pushed onto the stack.
	OpPush8       OpCode = 0x58 // The number 8 is pushed onto the stack.
	OpPush9       OpCode = 0x59 // The number 9 is pushed onto the stack.
	OpPush10      OpCode = 0x5A // The number 10 is pushed onto the stack.
	OpPush11      OpCode = 0x5B // The number 11 is pushed onto the stack.
	OpPush12      OpCode = 0x5C // The number 12 is pushed onto the stack.
	OpPush13      OpCode = 0x5D // The number 13 is pushed onto the stack.
	OpPush14      OpCode = 0x5E // The number 14 is pushed onto the stack.
	OpPush15      OpCode = 0x5F // The number 15 is pushed onto the stack.
	OpPush16      OpCode = 0x60 // The number 16 is pushed onto the stack.

	// Flow control
	OpNOP      OpCode = 0x61 // No operation.
	OpJMP      OpCode = 0x62
	OpJMPIF    OpCode = 0x63
	OpJMPIFNOT OpCode = 0x64
	OpCALL     OpCode = 0x65
	OpRET      OpCode = 0x66
	OpAPPCALL  OpCode = 0x67
	OpSYSCALL  OpCode = 0x68
	OpTAILCALL OpCode = 0x69

	// The stack
	OpDupFromAltStack OpCode = 0x6A
	OpToAltStack      OpCode = 0x6B // Puts the input onto the top of the alt stack. Removes it from the main stack.
	OpFromAltStack    OpCode = 0x6C // Puts the input onto the top of the main stack. Removes it from the alt stack.
	OpXDrop           OpCode = 0x6D
	OpXSwap           OpCode = 0x72
	OpXTuck           OpCode = 0x73
	OpDepth           OpCode = 0x74 // Puts the number of stack items onto the stack.
	OpDrop            OpCode = 0x75 // Removes the top stack item.
	OpDup             OpCode = 0x76 // Duplicates the top stack item.
	OpNip             OpCode = 0x77 // Removes the second-to-top stack item.
	OpOver            OpCode = 0x78 // Copies the second-to-top stack item to the top.
	OpPick            OpCode = 0x79 // The item n back in the stack is copied to the top.
	OpRoll            OpCode = 0x7A // The item n back in the stack is moved to the top.
	OpRot             OpCode = 0x7B // The top three items on the stack are rotated to the left.
	OpSwap            OpCode = 0x7C // The top two items on the stack are swapped.
	OpTuck            OpCode = 0x7D // The item at the top of the stack is copied and inserted before the second-to-top item.

	// Splice
	OpCat    OpCode = 0x7E // Concatenates two strings.
	OpSubStr OpCode = 0x7F // Returns a section of a string.
	OpLeft   OpCode = 0x80 // Keeps only characters left of the specified point in a string.
	OpRight  OpCode = 0x81 // Keeps only characters right of the specified point in a string.
	OpSize   OpCode = 0x82 // Returns the length of the input string.

	// Bitwise logic
	OpInvert OpCode = 0x83 // Flips all of the bits in the input.
	OpAnd    OpCode = 0x84 // Boolean and between each bit in the inputs.
	OpOr     OpCode = 0x85 // Boolean or between each bit in the inputs.
	OpXor    OpCode = 0x86 // Boolean exclusive or between each bit in the inputs.
	OpEqual  OpCode = 0x87 // Returns 1 if the inputs are exactly equal, 0 otherwise.

	// Arithmetic
	// Note: Arithmetic inputs are limited to signed 32-bit integers, but may overflow their output.
	OpInc         OpCode = 0x8B // 1 is added to the input.
	OpDec         OpCode = 0x8C // 1 is subtracted from the input.
	OpSign        OpCode = 0x8D
	OpNegate      OpCode = 0x8F // The sign of the input is flipped.
	OpAbs         OpCode = 0x90 // The input is made positive.
	OpNot         OpCode = 0x91 // If the input is 0 or 1, it is flipped. Otherwise the output will be 0.
	OpNZ          OpCode = 0x92 // Returns 0 if the input is 0. 1 otherwise.
	OpAdd         OpCode = 0x93 // a is added to b.
	OpSub         OpCode = 0x94 // b is subtracted from a.
	OpMul         OpCode = 0x95 // a is multiplied by b.
	OpDiv         OpCode = 0x96 // a is divided by b.
	OpMod         OpCode = 0x97 // Returns the remainder after dividing a by b.
	OpShl         OpCode = 0x98 // Shifts a left b bits, preserving sign.
	OpShr         OpCode = 0x99 // Shifts a right b bits, preserving sign.
	OpBoolAnd     OpCode = 0x9A // If both a and b are not 0, the output is 1. Otherwise 0.
	OpBoolOr      OpCode = 0x9B // If a or b is not 0, the output is 1. Otherwise 0.
	OpNumEqual    OpCode = 0x9C // Returns 1 if the numbers are equal, 0 otherwise.
	OpNumNotEqual OpCode = 0x9E // Returns 1 if the numbers are not equal, 0 otherwise.
	OpLT          OpCode = 0x9F // Returns 1 if a is less than b, 0 otherwise.
	OpGT          OpCode = 0xA0 // Returns 1 if a is greater than b, 0 otherwise.
	OpLTE         OpCode = 0xA1 // Returns 1 if a is less than or equal to b, 0 otherwise.
	OpGTE         OpCode = 0xA2 // Returns 1 if a is greater than or equal to b, 0 otherwise.
	OpMin         OpCode = 0xA3 // Returns the smaller of a and b.
	OpMax         OpCode = 0xA4 // Returns the larger of a and b.
	OpWithin      OpCode = 0xA5 // Returns 1 if x is within the specified range (left-inclusive), 0 otherwise.

	// Crypto
	OpSHA1          OpCode = 0xA7 // The input is hashed using SHA-1.
	OpSHA256        OpCode = 0xA8 // The input is hashed using SHA-256.
	OpHASH160       OpCode = 0xA9
	OpHASH256       OpCode = 0xAA
	OpCheckSig      OpCode = 0xAC
	OpCheckMultiSig OpCode = 0xAE

	// Array
	OpArraySize OpCode = 0xC0
	OpPack      OpCode = 0xC1
	OpUnpack    OpCode = 0xC2
	OpPickItem  OpCode = 0xC3
	OpSetItem   OpCode = 0xC4
	OpNewArray  OpCode = 0xC5
	OpNewStruct OpCode = 0xC6
	OpAppend    OpCode = 0xC8
	OpReverse   OpCode = 0xC9
	OpRemove    OpCode = 0xCA

	// Exceptions
	OpThrow      OpCode = 0xF0
	OpThrowIfNot OpCode = 0xF1
)
