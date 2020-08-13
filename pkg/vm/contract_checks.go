package vm

import (
	"encoding/binary"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var (
	verifyInteropID   = interopnames.ToID([]byte("Neo.Crypto.VerifyWithECDsaSecp256r1"))
	multisigInteropID = interopnames.ToID([]byte("Neo.Crypto.CheckMultisigWithECDsaSecp256r1"))
)

func getNumOfThingsFromInstr(instr opcode.Opcode, param []byte) (int, bool) {
	var nthings int

	switch {
	case opcode.PUSH1 <= instr && instr <= opcode.PUSH16:
		nthings = int(instr-opcode.PUSH1) + 1
	case instr <= opcode.PUSHINT256:
		n := bigint.FromBytes(param)
		if !n.IsInt64() || n.Int64() > stackitem.MaxArraySize {
			return 0, false
		}
		nthings = int(n.Int64())
	default:
		return 0, false
	}
	if nthings < 1 || nthings > stackitem.MaxArraySize {
		return 0, false
	}
	return nthings, true
}

// IsMultiSigContract checks whether the passed script is a multi-signature
// contract.
func IsMultiSigContract(script []byte) bool {
	_, _, ok := ParseMultiSigContract(script)
	return ok
}

// ParseMultiSigContract returns number of signatures and list of public keys
// from the verification script of the contract.
func ParseMultiSigContract(script []byte) (int, [][]byte, bool) {
	var nsigs, nkeys int

	ctx := NewContext(script)
	instr, param, err := ctx.Next()
	if err != nil {
		return nsigs, nil, false
	}
	nsigs, ok := getNumOfThingsFromInstr(instr, param)
	if !ok {
		return nsigs, nil, false
	}
	var pubs [][]byte
	for {
		instr, param, err = ctx.Next()
		if err != nil {
			return nsigs, nil, false
		}
		if instr != opcode.PUSHDATA1 {
			break
		}
		if len(param) < 33 {
			return nsigs, nil, false
		}
		pubs = append(pubs, param)
		nkeys++
		if nkeys > stackitem.MaxArraySize {
			return nsigs, nil, false
		}
	}
	if nkeys < nsigs {
		return nsigs, nil, false
	}
	nkeys2, ok := getNumOfThingsFromInstr(instr, param)
	if !ok {
		return nsigs, nil, false
	}
	if nkeys2 != nkeys {
		return nsigs, nil, false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != opcode.PUSHNULL {
		return nsigs, nil, false
	}
	instr, param, err = ctx.Next()
	if err != nil || instr != opcode.SYSCALL || binary.LittleEndian.Uint32(param) != multisigInteropID {
		return nsigs, nil, false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != opcode.RET || ctx.ip != len(script) {
		return nsigs, nil, false
	}
	return nsigs, pubs, true
}

// IsSignatureContract checks whether the passed script is a signature check
// contract.
func IsSignatureContract(script []byte) bool {
	if len(script) != 41 {
		return false
	}

	ctx := NewContext(script)
	instr, param, err := ctx.Next()
	if err != nil || instr != opcode.PUSHDATA1 || len(param) != 33 {
		return false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != opcode.PUSHNULL {
		return false
	}
	instr, param, err = ctx.Next()
	if err != nil || instr != opcode.SYSCALL || binary.LittleEndian.Uint32(param) != verifyInteropID {
		return false
	}
	return true
}

// IsStandardContract checks whether the passed script is a signature or
// multi-signature contract.
func IsStandardContract(script []byte) bool {
	return IsSignatureContract(script) || IsMultiSigContract(script)
}
