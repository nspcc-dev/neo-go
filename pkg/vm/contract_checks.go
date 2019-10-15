package vm

import (
	"encoding/binary"
)

func getNumOfThingsFromInstr(instr Instruction, param []byte) (int, bool) {
	var nthings int

	switch instr {
	case PUSH1, PUSH2, PUSH3, PUSH4, PUSH5, PUSH6, PUSH7, PUSH8,
		PUSH9, PUSH10, PUSH11, PUSH12, PUSH13, PUSH14, PUSH15, PUSH16:
		nthings = int(instr-PUSH1) + 1
	case PUSHBYTES1:
		nthings = int(param[0])
	case PUSHBYTES2:
		nthings = int(binary.LittleEndian.Uint16(param))
	default:
		return 0, false
	}
	if nthings < 1 || nthings > MaxArraySize {
		return 0, false
	}
	return nthings, true
}

// IsMultiSigContract checks whether the passed script is a multi-signature
// contract.
func IsMultiSigContract(script []byte) bool {
	var nsigs, nkeys int

	ctx := NewContext(script)
	instr, param, err := ctx.Next()
	if err != nil {
		return false
	}
	nsigs, ok := getNumOfThingsFromInstr(instr, param)
	if !ok {
		return false
	}
	for {
		instr, param, err = ctx.Next()
		if err != nil {
			return false
		}
		if instr != PUSHBYTES33 {
			break
		}
		nkeys++
		if nkeys > MaxArraySize {
			return false
		}
	}
	if nkeys < nsigs {
		return false
	}
	nkeys2, ok := getNumOfThingsFromInstr(instr, param)
	if !ok {
		return false
	}
	if nkeys2 != nkeys {
		return false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != CHECKMULTISIG {
		return false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != RET || ctx.ip != len(script) {
		return false
	}
	return true
}

// IsSignatureContract checks whether the passed script is a signature check
// contract.
func IsSignatureContract(script []byte) bool {
	ctx := NewContext(script)
	instr, _, err := ctx.Next()
	if err != nil || instr != PUSHBYTES33 {
		return false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != CHECKSIG {
		return false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != RET || ctx.ip != len(script) {
		return false
	}
	return true
}

// IsStandardContract checks whether the passed script is a signature or
// multi-signature contract.
func IsStandardContract(script []byte) bool {
	return IsSignatureContract(script) || IsMultiSigContract(script)
}
