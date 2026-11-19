package scparser

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/util/bitfield"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxMultisigKeys is the maximum number of keys allowed for correct multisig contract.
	MaxMultisigKeys = 1024

	// maxStackSize is the maximum stack size of Neo VM. It's not imported from
	// the `vm` package to avoid cyclic dependency.
	maxStackSize = 2 * 1024
)

var (
	verifyInteropID   = interopnames.ToID([]byte(interopnames.SystemCryptoCheckSig))
	multisigInteropID = interopnames.ToID([]byte(interopnames.SystemCryptoCheckMultisig))
)

// MultiSigContract contains the required signature count, public keys and the
// opcodes that encode the signature count (MOpcode) and key count (NOpcode).
type MultiSigContract struct {
	NSigs      int
	PublicKeys [][]byte
	MOpcode    opcode.Opcode
	NOpcode    opcode.Opcode
}

func getNumOfThingsFromInstr(instr opcode.Opcode, param []byte) (int, bool) {
	nthings, err := GetInt64FromInstr(Instruction{Op: instr, Param: param})
	if err != nil {
		return 0, false
	}
	if nthings < 1 || nthings > MaxMultisigKeys {
		return 0, false
	}
	return int(nthings), true
}

// IsMultiSigContract checks whether the passed script is a multi-signature
// contract.
func IsMultiSigContract(script []byte) bool {
	_, _, ok := ParseMultiSigContract(script)
	return ok
}

// ParseMultiSigContract returns the number of signatures and a list of public keys
// from the verification script of the contract.
func ParseMultiSigContract(script []byte) (int, [][]byte, bool) {
	contract, ok := ParseMultiSigContractWithOpcodes(script)
	return contract.NSigs, contract.PublicKeys, ok
}

// ParseMultiSigContractWithOpcodes parses a multi-signature verification script,
// preserving the opcodes used to encode the signature and key counts.
func ParseMultiSigContractWithOpcodes(script []byte) (MultiSigContract, bool) {
	var (
		contract MultiSigContract
		nkeys    int
	)
	if len(script) < 42 {
		return contract, false
	}

	ctx := NewContext(script, 0)
	instr, param, err := ctx.Next()
	if err != nil {
		return contract, false
	}
	contract.MOpcode = instr
	nsigs, ok := getNumOfThingsFromInstr(instr, param)
	contract.NSigs = nsigs
	if !ok {
		return contract, false
	}
	var pubs [][]byte
	for {
		instr, param, err = ctx.Next()
		if err != nil {
			return contract, false
		}
		if instr != opcode.PUSHDATA1 {
			break
		}
		if len(param) < 33 {
			return contract, false
		}
		pubs = append(pubs, param)
		nkeys++
		if nkeys > MaxMultisigKeys {
			return contract, false
		}
	}
	if nkeys < nsigs {
		return contract, false
	}
	contract.NOpcode = instr
	nkeys2, ok := getNumOfThingsFromInstr(instr, param)
	if !ok {
		return contract, false
	}
	if nkeys2 != nkeys {
		return contract, false
	}
	instr, param, err = ctx.Next()
	if err != nil || instr != opcode.SYSCALL || binary.LittleEndian.Uint32(param) != multisigInteropID {
		return contract, false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != opcode.RET || ctx.IP() != len(script) {
		return contract, false
	}
	contract.PublicKeys = pubs
	return contract, true
}

// IsSignatureContract checks whether the passed script is a signature check
// contract.
func IsSignatureContract(script []byte) bool {
	_, ok := ParseSignatureContract(script)
	return ok
}

// ParseSignatureContract parses a simple signature contract and returns
// a public key.
func ParseSignatureContract(script []byte) ([]byte, bool) {
	if len(script) != 40 {
		return nil, false
	}

	// We don't use Context for this simple case, it's more efficient this way.
	if script[0] == byte(opcode.PUSHDATA1) && // PUSHDATA1
		script[1] == 33 && // with a public key parameter
		script[35] == byte(opcode.SYSCALL) && // and a CheckSig SYSCALL.
		binary.LittleEndian.Uint32(script[36:]) == verifyInteropID {
		return script[2:35], true
	}
	return nil, false
}

// IsStandardContract checks whether the passed script is a signature or
// multi-signature contract.
func IsStandardContract(script []byte) bool {
	return IsSignatureContract(script) || IsMultiSigContract(script)
}

// IsScriptCorrect checks the script for errors and mask provided for correctness wrt
// instruction boundaries. Normally, it returns nil, but it can return some specific
// error if there is any.
func IsScriptCorrect(script []byte, methods bitfield.Field) error {
	var (
		l      = len(script)
		instrs = bitfield.New(l)
		jumps  = bitfield.New(l)
	)
	ctx := NewContext(script, 0)
	for ctx.NextIP() < l {
		op, param, err := ctx.Next()
		if err != nil {
			return err
		}
		instrs.Set(ctx.IP())
		switch op {
		case opcode.JMP, opcode.JMPIF, opcode.JMPIFNOT, opcode.JMPEQ, opcode.JMPNE,
			opcode.JMPGT, opcode.JMPGE, opcode.JMPLT, opcode.JMPLE,
			opcode.CALL, opcode.ENDTRY, opcode.JMPL, opcode.JMPIFL,
			opcode.JMPIFNOTL, opcode.JMPEQL, opcode.JMPNEL,
			opcode.JMPGTL, opcode.JMPGEL, opcode.JMPLTL, opcode.JMPLEL,
			opcode.ENDTRYL, opcode.CALLL, opcode.PUSHA:
			off, _, err := ctx.CalcJumpOffset(param)
			if err != nil {
				return err
			}
			// `calcJumpOffset` does bounds checking but can return `len(script)`.
			// This check avoids panic in bitset when script length is a multiple of 64.
			if off != len(script) {
				jumps.Set(off)
			}
		case opcode.TRY, opcode.TRYL:
			catchP, finallyP := GetTryParams(op, param)
			off, _, err := ctx.CalcJumpOffset(catchP)
			if err != nil {
				return err
			}
			if off != len(script) {
				jumps.Set(off)
			}
			off, _, err = ctx.CalcJumpOffset(finallyP)
			if err != nil {
				return err
			}
			if off != len(script) {
				jumps.Set(off)
			}
		case opcode.NEWARRAYT, opcode.ISTYPE, opcode.CONVERT:
			typ := stackitem.Type(param[0])
			if !typ.IsValid() {
				return fmt.Errorf("invalid type specification at offset %d", ctx.IP())
			}
			if typ == stackitem.AnyT && op != opcode.NEWARRAYT {
				return fmt.Errorf("using type ANY is incorrect at offset %d", ctx.IP())
			}
		default:
		}
	}
	if !jumps.IsSubset(instrs) {
		return errors.New("some jumps are done to wrong offsets (not to instruction boundary)")
	}
	if methods != nil && !methods.IsSubset(instrs) {
		return errors.New("some methods point to wrong offsets (not to instruction boundary)")
	}
	return nil
}

// GetTryParams splits [opcode.TRY] and [opcode.TRYL] instruction parameter
// into offsets for catch and finally blocks.
func GetTryParams(op opcode.Opcode, p []byte) ([]byte, []byte) {
	i := 1
	if op == opcode.TRYL {
		i = 4
	}
	return p[:i], p[i:]
}
