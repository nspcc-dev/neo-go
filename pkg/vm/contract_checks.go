package vm

import (
	"encoding/binary"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

func getNumOfThingsFromInstr(instr opcode.Opcode, param []byte) (int, bool) {
	var nthings int

	switch instr {
	case opcode.PUSH1, opcode.PUSH2, opcode.PUSH3, opcode.PUSH4,
		opcode.PUSH5, opcode.PUSH6, opcode.PUSH7, opcode.PUSH8,
		opcode.PUSH9, opcode.PUSH10, opcode.PUSH11, opcode.PUSH12,
		opcode.PUSH13, opcode.PUSH14, opcode.PUSH15, opcode.PUSH16:
		nthings = int(instr-opcode.PUSH1) + 1
	case opcode.PUSHBYTES1:
		nthings = int(param[0])
	case opcode.PUSHBYTES2:
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
		if instr != opcode.PUSHBYTES33 {
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
	if err != nil || instr != opcode.CHECKMULTISIG {
		return false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != opcode.RET || ctx.ip != len(script) {
		return false
	}
	return true
}

// IsSignatureContract checks whether the passed script is a signature check
// contract.
func IsSignatureContract(script []byte) bool {
	ctx := NewContext(script)
	instr, _, err := ctx.Next()
	if err != nil || instr != opcode.PUSHBYTES33 {
		return false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != opcode.CHECKSIG {
		return false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != opcode.RET || ctx.ip != len(script) {
		return false
	}
	return true
}

// IsStandardContract checks whether the passed script is a signature or
// multi-signature contract.
func IsStandardContract(script []byte) bool {
	return IsSignatureContract(script) || IsMultiSigContract(script)
}
