package scparser

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/bitfield"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxMultisigKeys is the maximum number of keys allowed for correct multisig contract.
	MaxMultisigKeys = 1024

	// MaxStackSize is the maximum stack size ov Neo VM.
	MaxStackSize = 2 * 1024 // TODO: cyclic `vm` package dependency.
)

var (
	verifyInteropID   = interopnames.ToID([]byte(interopnames.SystemCryptoCheckSig))
	multisigInteropID = interopnames.ToID([]byte(interopnames.SystemCryptoCheckMultisig))
)

func getNumOfThingsFromInstr(instr opcode.Opcode, param []byte) (int, bool) {
	nthings, ok := GetIntFromInstr(instr, param)
	if !ok {
		return 0, false
	}
	if nthings < 1 || nthings > MaxMultisigKeys {
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

// ParseMultiSigContract returns the number of signatures and a list of public keys
// from the verification script of the contract.
func ParseMultiSigContract(script []byte) (int, [][]byte, bool) {
	var nsigs, nkeys int
	if len(script) < 42 {
		return nsigs, nil, false
	}

	ctx := NewContext(script, 0)
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
		if nkeys > MaxMultisigKeys {
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
	instr, param, err = ctx.Next()
	if err != nil || instr != opcode.SYSCALL || binary.LittleEndian.Uint32(param) != multisigInteropID {
		return nsigs, nil, false
	}
	instr, _, err = ctx.Next()
	if err != nil || instr != opcode.RET || ctx.IP() != len(script) {
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
			if typ == stackitem.AnyT && op != opcode.NEWARRAYT { // TODO: condition is always false?
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

// GetIntFromInstr returns the integer value emitted by the specified
// instruction. It works with static integer values only; for the rest of cases
// (result of SYSCALL, custom integer operations, etc.) parse the script
// manually.
func GetIntFromInstr(instr opcode.Opcode, param []byte) (int, bool) {
	var i int

	switch {
	case opcode.PUSH1 <= instr && instr <= opcode.PUSH16:
		i = int(instr-opcode.PUSH1) + 1
	case instr <= opcode.PUSHINT256:
		n := bigint.FromBytes(param)
		if !n.IsInt64() {
			return 0, false
		}
		i = int(n.Int64())
	default:
		return 0, false
	}

	return i, true
}

// GetStringFromInstr returns the string value emitted by the specified
// instruction. It works with static string values only; for the rest of cases
// (result of SYSCALL, custom string operations, etc.) parse the script
// manually.
func GetStringFromInstr(instr opcode.Opcode, param []byte) (string, bool) {
	var s string

	switch {
	case opcode.PUSHDATA1 <= instr && instr <= opcode.PUSHDATA4:
		s = string(param)
	default:
		return s, false
	}

	return s, true
}

// GetBoolFromInstr returns the boolean value emitted by the specified
// instruction. It works with static bool values only; for the rest of cases
// (result of SYSCALL, custom bool operations, etc.) parse the script
// manually.
func GetBoolFromInstr(instr opcode.Opcode, param []byte) (bool, bool) {
	var b bool
	if len(param) != 0 {
		return b, false
	}

	switch instr {
	case opcode.PUSHF, opcode.PUSH0:
	case opcode.PUSHT, opcode.PUSH1:
		b = true
	default:
		return b, false
	}

	return b, true
}

// GetUint160FromInstr returns the [util.Uint160] value emitted by the specified
// instruction. It works with static hash values only; for the rest of cases
// (result of SYSCALL, custom bytes operations, etc.) parse the script
// manually.
//
// TODO: @roman-khimov, I'm thinking about returning `error` instead of `bool` in these helpers,
//
//	it's more representative.
//	Originally `bool` was done to match existing ParseMultiSigContract/etc helpers. ACK?
func GetUint160FromInstr(instr opcode.Opcode, param []byte) (util.Uint160, bool) {
	var u util.Uint160

	switch instr {
	case opcode.PUSHDATA1:
		var err error
		u, err = util.Uint160DecodeBytesBE(param)
		if err != nil {
			return u, false
		}
		return u, true
	default:
		return u, false
	}
}

// GetUint256FromInstr returns the [util.Uint256] value emitted by the specified
// instruction. It works with static hash values only; for the rest of cases
// (result of SYSCALL, custom bytes operations, etc.) parse the script
// manually.
func GetUint256FromInstr(instr opcode.Opcode, param []byte) (util.Uint256, bool) {
	var u util.Uint256

	switch instr {
	case opcode.PUSHDATA1:
		var err error
		u, err = util.Uint256DecodeBytesBE(param)
		if err != nil {
			return u, false
		}
		return u, true
	default:
		return u, false
	}
}

// GetEFromInstr is a delegate that returns an element emitted by the specified
// instruction and the OK indicator. [GetIntFromInstr], [GetStringFromInstr],
// [GetBoolFromInstr], [GetUint160FromInstr], [GetUint256FromInstr] are suitable
// implementations.
type GetEFromInstr[E string | int | bool | util.Uint160 | util.Uint256] func(instr opcode.Opcode, param []byte) (E, bool)

// ParseList parses a list of elements from the VM context. It modifies the
// given context, so save the current context's IP before calling ParseList
// to be able to reset the context state afterwards. ParseList accepts
// [GetEFromInstr] delegate to retrieve a single list element from the VM
// context. [MaxStackSize] is used as the default list length constraint if
// maxLen is not specified.
//
// TODO: @roman-khimov, I have two questions:
//  1. Replace `ctx *Context` argument with `script []byte` to follow ParseMultiSigContract signature?
//  2. Define (*Context).ParseList API (and a set of other useful APIs) to reuse them from scparser.Parse* ?
func ParseList[E string | int | bool](ctx *Context, getEFromInstr GetEFromInstr[E], maxLen ...int) ([]E, error) {
	var (
		res   []E
		instr opcode.Opcode
		param []byte
		err   error
	)

	maxL := MaxStackSize
	if len(maxLen) > 0 {
		maxL = maxLen[0]
	}

	for l := 1; ; l++ {
		instr, param, err = ctx.Next()
		if err != nil {
			return res, fmt.Errorf("failed to parse list element %d: %w", l-1, err)
		}
		e, ok := getEFromInstr(instr, param)
		if !ok {
			if l == 1 && instr == opcode.NEWARRAY0 {
				res = make([]E, 0)
				return res, nil
			}
			break
		}
		res = append(res, e)
		if l > maxL {
			return res, fmt.Errorf("number of elements exceeds %d", maxL)
		}
	}
	l, ok := GetIntFromInstr(instr, param)
	if !ok {
		return res, fmt.Errorf("failed to parse list length: not an int")
	}
	if len(res) != l {
		return res, fmt.Errorf("list length doesn't match PACK argument: %d vs %d", len(res), l)
	}
	instr, param, err = ctx.Next()
	if err != nil {
		return res, fmt.Errorf("failed to parse PACK: %w", err)
	}
	if instr != opcode.PACK {
		return res, fmt.Errorf("expected PACK, got %s", instr)
	}
	if len(param) != 0 {
		return res, fmt.Errorf("additional parameter got after PACK: %s", hex.EncodeToString(param))
	}
	slices.Reverse(res)

	return res, nil
}

// ParseAppCallNoArgs parses a contract call (which is effectively an
// [opcode.SYSCALL] instruction with [interopnames.SystemContractCall]
// parameter) assuming the arguments of the contract call (usually followed by
// [opcode.PACK] opcode) are already parsed. It returns the calling contract
// hash, method name and callflags.
func ParseAppCallNoArgs(ctx *Context) (util.Uint160, string, callflag.CallFlag, error) {
	instr, param, err := ctx.Next()
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse callflag instruction: %w", err)
	}
	f, ok := GetIntFromInstr(instr, param)
	if !ok {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse callflag: not an int")
	}
	if !callflag.IsValid(callflag.CallFlag(f)) {
		return util.Uint160{}, "", 0, fmt.Errorf("invalid callflag value: %d", f)
	}
	instr, param, err = ctx.Next()
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse method instruction: %w", err)
	}
	method, ok := GetStringFromInstr(instr, param)
	if !ok {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse method: not a string")
	}
	instr, param, err = ctx.Next()
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse contract scripthash instruction: %w", err)
	}
	h, ok := GetUint160FromInstr(instr, param)
	if !ok {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse contract scripthash: not a hash")
	}
	instr, param, err = ctx.Next()
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse SYSCALL instruction: %w", err)
	}
	if instr != opcode.SYSCALL {
		return util.Uint160{}, "", 0, fmt.Errorf("expected SYSCALL, got %s", instr)
	}
	if len(param) != 4 {
		return util.Uint160{}, "", 0, fmt.Errorf("expected 4 bytes SYSCALL parameter, got %d", len(param))
	}
	name, err := interopnames.FromID(binary.LittleEndian.Uint32(param))
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse SYSCALL parameter: %w", err)
	}
	if name != interopnames.SystemContractCall {
		return util.Uint160{}, "", 0, fmt.Errorf("expected SystemContractCall SYSCALL, got %s", name)
	}
	return h, method, callflag.CallFlag(f), nil
}
