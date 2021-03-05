package vm

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/util/bitfield"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var (
	verifyInteropID   = interopnames.ToID([]byte(interopnames.NeoCryptoCheckSig))
	multisigInteropID = interopnames.ToID([]byte(interopnames.NeoCryptoCheckMultisigWithECDsaSecp256r1))
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
	_, ok := ParseSignatureContract(script)
	return ok
}

// ParseSignatureContract parses simple signature contract and returns
// public key.
func ParseSignatureContract(script []byte) ([]byte, bool) {
	if len(script) != 40 {
		return nil, false
	}

	ctx := NewContext(script)
	instr, param, err := ctx.Next()
	if err != nil || instr != opcode.PUSHDATA1 || len(param) != 33 {
		return nil, false
	}
	pub := param
	instr, param, err = ctx.Next()
	if err != nil || instr != opcode.SYSCALL || binary.LittleEndian.Uint32(param) != verifyInteropID {
		return nil, false
	}
	return pub, true
}

// IsStandardContract checks whether the passed script is a signature or
// multi-signature contract.
func IsStandardContract(script []byte) bool {
	return IsSignatureContract(script) || IsMultiSigContract(script)
}

// IsScriptCorrect checks script for errors and mask provided for correctness wrt
// instruction boundaries. Normally it returns nil, but can return some specific
// error if there is any.
func IsScriptCorrect(script []byte, methods bitfield.Field) error {
	var (
		l      = len(script)
		instrs = bitfield.New(l)
		jumps  = bitfield.New(l)
	)
	ctx := NewContext(script)
	for ctx.nextip < l {
		op, param, err := ctx.Next()
		if err != nil {
			return err
		}
		instrs.Set(ctx.ip)
		switch op {
		case opcode.JMP, opcode.JMPIF, opcode.JMPIFNOT, opcode.JMPEQ, opcode.JMPNE,
			opcode.JMPGT, opcode.JMPGE, opcode.JMPLT, opcode.JMPLE,
			opcode.CALL, opcode.ENDTRY, opcode.JMPL, opcode.JMPIFL,
			opcode.JMPIFNOTL, opcode.JMPEQL, opcode.JMPNEL,
			opcode.JMPGTL, opcode.JMPGEL, opcode.JMPLTL, opcode.JMPLEL,
			opcode.ENDTRYL, opcode.CALLL, opcode.PUSHA:
			off, _, err := calcJumpOffset(ctx, param) // It does bounds checking.
			if err != nil {
				return err
			}
			jumps.Set(off)
		case opcode.TRY, opcode.TRYL:
			catchP, finallyP := getTryParams(op, param)
			off, _, err := calcJumpOffset(ctx, catchP)
			if err != nil {
				return err
			}
			jumps.Set(off)
			off, _, err = calcJumpOffset(ctx, finallyP)
			if err != nil {
				return err
			}
			jumps.Set(off)
		case opcode.NEWARRAYT, opcode.ISTYPE, opcode.CONVERT:
			typ := stackitem.Type(param[0])
			if !typ.IsValid() {
				return fmt.Errorf("invalid type specification at offset %d", ctx.ip)
			}
			if typ == stackitem.AnyT && op != opcode.NEWARRAYT {
				return fmt.Errorf("using type ANY is incorrect at offset %d", ctx.ip)
			}
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
