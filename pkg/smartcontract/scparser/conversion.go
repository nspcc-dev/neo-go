package scparser

import (
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// PushedItem represents a container for VM instruction (opcode with
// corresponding parameter) potentially emitting stackitem to stack. If
// instruction execution results in some nested stackitem emission (Array, Map
// or Struct), then List/Map is filled in with the content of this stakitem (see
// the [GetListFromContext] documentation for the list of VM opcodes
// corresponding to such instructions).
type PushedItem struct {
	Instruction

	List []PushedItem
	Map  []MapPair
}

// Instruction represents a single VM instruction ([opcode.Opcode] with an
// optional parameter). Do not modify the content of Param since no copy
// of the original script buffer is created during script parsing.
type Instruction struct {
	Op    opcode.Opcode
	Param []byte
}

// MapPair represents a key-value pair in a Map stackitem. Key is always
// represented as a single Instruction and can only emit [stackitem.Boolean],
// [stackitem.Integer] or [stackitem.ByteString] on stack. Value is a PushedItem
// that may emit as primitive as complex stackitems.
type MapPair struct {
	Key   Instruction
	Value PushedItem
}

// IsNested denotes whether execution of this instruction results in emission of
// some nested stackitem (Array, Struct or Map). See the [GetListFromContext]
// documentation for the list of VM opcodes corresponding to such instructions.
func (i PushedItem) IsNested() bool {
	return i.IsList() || i.IsMap()
}

// IsList denotes whether the underlying PushedItem is a list (pushes
// [stackitem.Array] or [stackitem.Struct] on stack).
func (i PushedItem) IsList() bool {
	return i.List != nil
}

// IsMap denotes whether the underlying PushedItem is a map (pushes
// [stackitem.Map] on stack).
func (i PushedItem) IsMap() bool {
	return i.Map != nil
}

// IsEmpty denotes whether this instruction is a stub used to fill in the gaps
// in non-strict mode of script parsing. Such empty instructions may be produced
// by [GetListFromContext] and similar methods.
func (i PushedItem) IsEmpty() bool {
	return i.Op == 0 && i.Param == nil && i.List == nil && i.Map == nil
}

// IsNull returns true if the underlying instruction pushes [stackitem.Null] on
// stack.
func (i PushedItem) IsNull() bool {
	return !i.IsNested() && i.Op == opcode.PUSHNULL
}

// GetEFromInstr is a delegate that returns an element emitted by the specified
// [Instruction] and the error. [GetInt64FromInstr], [GetBigIntFromInstr],
// [GetStringFromInstr], [GetUTF8StringFromInstr], [GetBoolFromInstr],
// [GetUint160FromInstr],[GetUint256FromInstr], [GetSignatureFromInstr],
// [GetPublicKeyFromInstr] are suitable implementations. You may also use a
// custom implementation.
type GetEFromInstr[E any] func(Instruction) (E, error)

// GetInt64FromInstr returns int64 value (ensuring bounds) emitted by the
// specified instruction. It works with static integer values only; for the rest
// of cases (result of SYSCALL, custom integer operations, etc.) parse the
// script manually. It returns [ErrEmptyInstruction] on attempt to convert an
// empty instruction got after non-strict script parsing.
func GetInt64FromInstr(instr Instruction) (int64, error) {
	switch {
	case opcode.PUSHM1 <= instr.Op && instr.Op <= opcode.PUSH16:
		return int64(instr.Op) - int64(opcode.PUSH0), nil
	case instr.Op == opcode.PUSHINT8:
		return int64(int8(instr.Param[0])), nil
	case instr.Op == opcode.PUSHINT16:
		return int64(int16(binary.LittleEndian.Uint16(instr.Param))), nil
	case instr.Op == opcode.PUSHINT32:
		return int64(int32(binary.LittleEndian.Uint32(instr.Param))), nil
	case instr.Op == opcode.PUSHINT64:
		return int64(binary.LittleEndian.Uint64(instr.Param)), nil
	case instr.Op == opcode.PUSHINT128 || instr.Op == opcode.PUSHINT256:
		for _, b := range instr.Param[8:] {
			if b != 0 {
				return 0, errors.New("parameter is not int64")
			}
		}
		if byte(instr.Param[7]) >= 0x80 {
			return 0, errors.New("parameter is not int64")
		}

		return int64(binary.LittleEndian.Uint64(instr.Param)), nil
	default:
		return 0, fmt.Errorf("expected %s, %s-%s, %s-%s got %s", opcode.PUSHM1, opcode.PUSH0, opcode.PUSH16, opcode.PUSHINT8, opcode.PUSHINT256, instr)
	}
}

// GetBigIntFromInstr returns *big.Int value emitted by the specified
// instruction. It works with static integer values only; for the rest of cases
// (result of SYSCALL, custom integer operations, etc.) parse the script
// manually. It returns [ErrEmptyInstruction] on attempt to convert an empty
// instruction got after non-strict script parsing.
func GetBigIntFromInstr(instr Instruction) (*big.Int, error) {
	switch {
	case opcode.PUSHM1 <= instr.Op && instr.Op <= opcode.PUSH16:
		return big.NewInt(int64(instr.Op) - int64(opcode.PUSH0)), nil
	case instr.Op <= opcode.PUSHINT256:
		return bigint.FromBytes(instr.Param), nil
	default:
		return nil, fmt.Errorf("expected %s, %s-%s, %s-%s got %s", opcode.PUSHM1, opcode.PUSH0, opcode.PUSH16, opcode.PUSHINT8, opcode.PUSHINT256, instr)
	}
}

// GetStringFromInstr returns the string value emitted by the specified
// instruction. It works with static string values only; for the rest of cases
// (result of SYSCALL, custom string operations, etc.) parse the script
// manually. It returns [ErrEmptyInstruction] on attempt to convert an empty
// instruction got after non-strict script parsing.
func GetStringFromInstr(instr Instruction) (string, error) {
	return getStringFromInstr(instr)
}

// GetUTF8StringFromInstr returns the string value emitted by the specified
// instruction. It checks that the result is a valid UTF-8 string. It works
// with static string values only; for the rest of cases (result of SYSCALL,
// custom string operations, etc.) parse the script manually. It returns
// [ErrEmptyInstruction] on attempt to convert an empty instruction got after
// non-strict script parsing.
func GetUTF8StringFromInstr(instr Instruction) (string, error) {
	s, err := getStringFromInstr(instr)
	if err != nil {
		return "", err
	}
	if !utf8.Valid([]byte(s)) {
		return "", errors.New("not a UTF-8 string")
	}
	return s, nil
}

func getStringFromInstr(instr Instruction) (string, error) {
	s, err := GetBytesFromInstr(instr)
	return string(s), err
}

// GetBytesFromInstr returns bytes value emitted by the specified PUSHDATA*
// instruction. It works with static byte values only; for the rest of cases
// (result of SYSCALL, custom bytes operations, etc.) parse the script
// manually. It returns [ErrEmptyInstruction] on attempt to convert an empty
// instruction got after non-strict script parsing.
func GetBytesFromInstr(instr Instruction) ([]byte, error) {
	switch {
	case opcode.PUSHDATA1 <= instr.Op && instr.Op <= opcode.PUSHDATA4:
		return instr.Param, nil
	default:
		return nil, fmt.Errorf("expected %s, %s, %s, got %s", opcode.PUSHDATA1, opcode.PUSHDATA2, opcode.PUSHDATA4, instr)
	}
}

// GetBoolFromInstr returns the boolean value emitted by the specified
// instruction. It works with static bool values only; for the rest of cases
// (result of SYSCALL, custom bool operations, etc.) parse the script
// manually. It returns [ErrEmptyInstruction] on attempt to convert an empty
// instruction got after non-strict script parsing.
func GetBoolFromInstr(instr Instruction) (bool, error) {
	switch instr.Op {
	case opcode.PUSHF, opcode.PUSH0:
		return false, nil
	case opcode.PUSHT, opcode.PUSH1:
		return true, nil
	default:
		return false, fmt.Errorf("expected %s, %s, %s, %s, got %s", opcode.PUSHF, opcode.PUSH0, opcode.PUSHT, opcode.PUSH1, instr)
	}
}

// GetUint160FromInstr returns the [util.Uint160] value emitted by the specified
// instruction. It works with static hash values only; for the rest of cases
// (result of SYSCALL, custom bytes operations, etc.) parse the script
// manually. It returns [ErrEmptyInstruction] on attempt to convert an empty
// instruction got after non-strict script parsing.
func GetUint160FromInstr(instr Instruction) (util.Uint160, error) {
	return getHashFromInstr(instr, util.Uint160DecodeBytesBE)
}

// GetUint256FromInstr returns the [util.Uint256] value emitted by the specified
// instruction. It works with static hash values only; for the rest of cases
// (result of SYSCALL, custom bytes operations, etc.) parse the script
// manually. It returns [ErrEmptyInstruction] on attempt to convert an empty
// instruction got after non-strict script parsing.
func GetUint256FromInstr(instr Instruction) (util.Uint256, error) {
	return getHashFromInstr(instr, util.Uint256DecodeBytesBE)
}

// GetSignatureFromInstr returns the standard signature bytes emitted by the
// specified instruction. It works with static values only; for the rest of
// cases (result of SYSCALL, custom bytes operations, etc.) parse the script
// manually. It returns [ErrEmptyInstruction] on attempt to convert an empty
// instruction got after non-strict script parsing.
func GetSignatureFromInstr(instr Instruction) ([]byte, error) {
	return getHashFromInstr[[]byte](instr, func(b []byte) ([]byte, error) {
		if len(b) != keys.SignatureLen {
			return nil, fmt.Errorf("failed to decode signature from instruction: expected %d bytes, got %d", keys.SignatureLen, len(instr.Param))
		}
		return b, nil
	})
}

// GetPublicKeyFromInstr returns the secp256r1 public key bytes emitted by the
// specified instruction. It works with static values only; for the rest of
// cases (result of SYSCALL, custom bytes operations, etc.) parse the script
// manually. It returns nil in case if instruction is empty. It returns
// [ErrEmptyInstruction] on attempt to convert an empty instruction got after
// non-strict script parsing.
func GetPublicKeyFromInstr(instr Instruction) (*keys.PublicKey, error) {
	return getHashFromInstr[*keys.PublicKey](instr, func(b []byte) (*keys.PublicKey, error) {
		if len(b) != 33 {
			return nil, fmt.Errorf("failed to decode public key from instruction: expected 33 bytes, got %d", len(b))
		}
		return keys.NewPublicKeyFromBytes(b, elliptic.P256())
	})
}

func getHashFromInstr[E util.Uint160 | util.Uint256 | []byte | *keys.PublicKey](instr Instruction, hashFromBE func([]byte) (E, error)) (E, error) {
	switch instr.Op {
	case opcode.PUSHDATA1:
		u, err := hashFromBE(instr.Param)
		if err != nil {
			return u, err
		}
		return u, nil
	default:
		var u E
		return u, fmt.Errorf("expected %s, got %s", opcode.PUSHDATA1, instr)
	}
}

// GetListOfEFromPushedItem works like GetListOfEFromPushedItems except that it
// accepts a single [PushedItem] expecting it to be a list of [PushedItem] of
// the specified type.
func GetListOfEFromPushedItem[E any](item PushedItem, getEFromInstr GetEFromInstr[E]) ([]E, error) {
	if !item.IsList() {
		return nil, fmt.Errorf("expected list, got %s", item.Op)
	}
	return GetListOfEFromPushedItems(item.List, getEFromInstr)
}

// GetListOfEFromPushedItems returns the list of uniformly typed elements parsed
// from the provided list of [PushedItem]. It accepts [GetEFromInstr] delegate
// to retrieve a single list element from the [Instruction] (hence, only
// non-nested [PushedItem] types are supported; use a custom parser for the list
// of nested types). It returns [ErrEmptyInstruction] on attempt to convert an
// empty instruction got after non-strict script parsing.
func GetListOfEFromPushedItems[E any](items []PushedItem, getEFromInstr GetEFromInstr[E]) ([]E, error) {
	var (
		res = make([]E, len(items))
		err error
	)
	for i, instr := range items {
		res[i], err = getEFromInstr(instr.Instruction)
		if err != nil {
			return nil, fmt.Errorf("failed to parse element #%d: %w", i, err)
		}
	}
	return res, nil
}

// GetListOfEFromContext is an implementation of GetListFromContext that works
// with uniformly typed elements. It accepts [GetEFromInstr] delegate to
// retrieve a single list element from the [Instruction] (hence, only non-nested
// [PushedItem] types are supported; use a custom parser for the list of nested
// types). It returns [ErrEmptyInstruction] on attempt to convert an empty
// instruction got after non-strict script parsing.
func GetListOfEFromContext[E any](ctx *Context, getEFromInstr GetEFromInstr[E], maxLen ...int) ([]E, error) {
	list, err := GetListFromContext(ctx, maxLen...)
	if err != nil {
		return nil, err
	}

	return GetListOfEFromPushedItems(list, getEFromInstr)
}

// GetListFromContext parses a list of elements from the VM context starting
// from the beginning and ending with the first [opcode.SYSCALL] or [opcode.RET]
// occurrence or the program end. It modifies the given context, so save the
// current context's IP before calling GetListFromContext to be able to reset
// the context state afterwards. [vm.MaxStackSize] is used as the default list
// length constraint if maxLen is not specified. It works with nested
// Array/Struct/Map items produced by [opcode.NEWARRAY0], [opcode.NEWSTRUCT0],
// [opcode.PACK], [opcode.PACKSTRUCT], [opcode.PACKMAP]. Optional [opcode.SWAP],
// [opcode.REVERSE3] [opcode.REVERSE4], [opcode.REVERSEN], [opcode.REVERSEITEMS]
// instructions are supported. It does not support any other [opcode.SYSCALL]
// instructions except System.Contract.Call. Once finished, it resets ctx state
// to the instruction corresponding to System.Contract.Call's callflag argument
// (if there's any). It does not check if there are additional opcodes after the
// first [opcode.RET] occurrence, so check the context's IP against the script
// length if you need to ensure that. It returns an unwrapped list of elements
// in the direct order (no additional reverse due to stack operations order is
// required).
func GetListFromContext(ctx *Context, maxLen ...int) ([]PushedItem, error) {
	return getListFromContext(ctx, true, maxLen...)
}

// GetListFromContextNonStrict works exactly like GetListFromContext except that
// it does not check PACK/PACKSTRUCT/PACKMAP arguments against the stack length.
// Missing elements (if any) will be filled with empty PushedItem.
func GetListFromContextNonStrict(ctx *Context, maxLen ...int) ([]PushedItem, error) {
	return getListFromContext(ctx, false, maxLen...)
}

func getListFromContext(ctx *Context, strict bool, maxLen ...int) ([]PushedItem, error) {
	res, argsEndOffset, err := getSomethingFromContext(ctx, strict, func(res []PushedItem) ([]PushedItem, error) {
		if len(res) != 4 {
			return nil, fmt.Errorf("System.Contract.Call requires 4 parameters, got %d", len(res))
		}
		return res[:len(res)-3], nil // cut the arguments of System.Contract.Call.
	}, maxLen...)
	if err != nil {
		return nil, err
	}

	// Unwrap the resulting Array/Struct.
	if len(res) != 1 {
		return res, fmt.Errorf("unexpected number of elements on stack: expected 1 Array/Struct, got %d elements", len(res))
	}
	if !res[0].IsList() {
		return res, fmt.Errorf("expected Array/Struct on stack, got %s (isMap: %t)", res[0].Op, res[0].IsMap())
	}
	res = res[0].List

	// Reset the context to the instruction following PACK/NEWARRAY0/REVERSEITEMS to support
	// parsing a subsequent call to GetAppCallNoArgsFromContext with the same context.
	if 0 < argsEndOffset && argsEndOffset < ctx.LenInstr() {
		ctx.Jump(argsEndOffset)
	}
	return res, nil
}

// getSomethingFromContext parses anything that is pushed on stack. It returns
// the list of parsed items and a pointer to the instruction that is next to
// PACK/NEWARRAY0/REVERSEITEMS/SWAP/REVERSE*. The pointer may be used to support
// parsing a subsequent call to GetAppCallNoArgsFromContext with the same
// context.
func getSomethingFromContext(ctx *Context, strict bool, onSystemContractCall func(res []PushedItem) ([]PushedItem, error), maxLen ...int) ([]PushedItem, int, error) {
	var (
		res            = make([]PushedItem, 0)
		lastPackNextIP int // nextIP of the last PACK*/NEWARRAY0/NEWSTRUCT0/SWAP/REVERSE* instruction.
		progLen        = ctx.LenInstr()
	)
	maxL := maxStackSize
	if len(maxLen) > 0 {
		maxL = maxLen[0]
	}
parseLoop:
	for instr, param, err := ctx.Next(); ctx.IP() < progLen; instr, param, err = ctx.Next() {
		if err != nil {
			return res, 0, fmt.Errorf("failed to parse list element: %w", err)
		}
		switch instr {
		case opcode.NEWARRAY0, opcode.NEWSTRUCT0:
			e := make([]PushedItem, 0)
			res = append(res, PushedItem{
				Instruction: Instruction{
					Op:    instr,
					Param: param,
				},
				List: e,
			})

			lastPackNextIP = ctx.NextIP()
		case opcode.PACK, opcode.PACKSTRUCT, opcode.PACKMAP:
			if len(res) == 0 {
				return res, 0, fmt.Errorf("%s instruction requires at least 1 element on stack", instr)
			}
			// Previously parsed element is PACK's argument.
			packArg, err := GetInt64FromInstr(res[len(res)-1].Instruction)
			if err != nil {
				return res, 0, fmt.Errorf("failed to parse %s argument: %w", instr, err)
			}
			if packArg < 0 {
				return res, 0, fmt.Errorf("negative %s argument: %d", instr, packArg)
			}
			res = res[:len(res)-1]
			switch instr {
			case opcode.PACKMAP:
				packed := make([]MapPair, packArg)
				elementsNum := 2 * int(packArg)
				if strict && len(res) < elementsNum {
					return res, 0, fmt.Errorf("insufficient number of elements for %s: expected %d, has %d", instr, elementsNum, len(res))
				}
				toPack := min(len(res), elementsNum)
				for i := range toPack {
					// Reverse in-place, leave the last elements empty (or half-filled) if non-strict mode is enabled.
					e := res[len(res)-i-1]
					if i%2 == 0 {
						if e.IsNested() {
							return res, 0, fmt.Errorf("%s (%d): invalid map key %d: Array/Struct/Map", instr, ctx.IP(), i/2)
						}
						packed[i/2].Key = e.Instruction
					} else {
						packed[i/2].Value = e
					}
				}
				res = res[:len(res)-toPack]
				res = append(res, PushedItem{
					Instruction: Instruction{
						Op:    instr,
						Param: param,
					},
					Map: packed,
				})
			default:
				packed := make([]PushedItem, packArg)
				if strict && len(res) < int(packArg) {
					return res, 0, fmt.Errorf("insufficient number of elements for %s: expected %d, has %d", instr, packArg, len(res))
				}
				toPack := min(len(res), int(packArg))
				for i := range toPack {
					packed[i] = res[len(res)-i-1] // reverse in-place, leave last elements empty if non-strict mode is enabled.
				}
				res = res[:len(res)-toPack]
				res = append(res, PushedItem{
					Instruction: Instruction{
						Op:    instr,
						Param: param,
					},
					List: packed,
				})
			}

			lastPackNextIP = ctx.NextIP()
		case opcode.DUP:
			res = append(res, res[len(res)-1])
		case opcode.REVERSEITEMS:
			if len(res) == 0 {
				return res, 0, fmt.Errorf("REVERSEITEMS instruction requires at least 1 element on stack")
			}
			e := res[len(res)-1]
			res = res[:len(res)-1]
			switch {
			case e.IsList():
				slices.Reverse(e.List)
			default:
				return res, 0, fmt.Errorf("static parser supports only Array/Struct for REVERSEITEMS instruction, got %s", e.Op)
			}
			lastPackNextIP = ctx.NextIP()
		case opcode.SWAP, opcode.REVERSE3, opcode.REVERSE4, opcode.REVERSEN:
			n := 2
			switch instr {
			case opcode.REVERSE3:
				n = 3
			case opcode.REVERSE4:
				n = 4
			case opcode.REVERSEN:
				if len(res) == 0 {
					return res, 0, fmt.Errorf("REVERSEN instruction requires at least 1 element on stack")
				}
				i, err := GetInt64FromInstr(res[len(res)-1].Instruction)
				if err != nil {
					return res, 0, fmt.Errorf("failed to parse REVERSEN argument: %w", err)
				}
				n = int(i)
				res = res[:len(res)-1]
			default:
			}
			if err := reverseTop(n, res); err != nil {
				return res, 0, fmt.Errorf("failed to apply %s (%d elements): %w", instr, n, err)
			}

			lastPackNextIP = ctx.NextIP()
		case opcode.SYSCALL:
			// Only System.Contract.Call is supported for now.
			err := verifySystemContractCall(instr, param)
			if err != nil {
				return nil, 0, fmt.Errorf("static parser supports only System.Contract.Call SYSCALL: %w", err)
			}
			res, err = onSystemContractCall(res)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to parse SYSCALL: %w", err)
			}
			break parseLoop
		case opcode.NOP:
		case opcode.RET:
			break parseLoop
		case opcode.PUSHINT8, opcode.PUSHINT16, opcode.PUSHINT32, opcode.PUSHINT64, opcode.PUSHINT128, opcode.PUSHINT256,
			opcode.PUSHT, opcode.PUSHF,
			opcode.PUSHNULL,
			opcode.PUSHDATA1, opcode.PUSHDATA2, opcode.PUSHDATA4,
			opcode.PUSHM1, opcode.PUSH0, opcode.PUSH1, opcode.PUSH2, opcode.PUSH3,
			opcode.PUSH4, opcode.PUSH5, opcode.PUSH6, opcode.PUSH7,
			opcode.PUSH8, opcode.PUSH9, opcode.PUSH10, opcode.PUSH11,
			opcode.PUSH12, opcode.PUSH13, opcode.PUSH14, opcode.PUSH15, opcode.PUSH16:
			res = append(res, PushedItem{Instruction: Instruction{Op: instr, Param: param}}) // don't make a copy since no parameter modification is performed.
			if len(res) > maxL+1 {                                                           // 1 extra element is allowed for PACK's argument
				return res, 0, fmt.Errorf("number of elements exceeds %d", maxL)
			}
		default:
			return res, 0, fmt.Errorf("static parser does not support %s instruction", instr)
		}
	}
	return res, lastPackNextIP, nil
}

// reverseTop reverses top n items of the stack represented as a list (top
// elements are in the end).
func reverseTop[E any](n int, list []E) error {
	l := len(list)
	if n < 0 {
		return errors.New("negative index")
	} else if n > l {
		return errors.New("too big index")
	} else if n <= 1 {
		return nil
	}

	slices.Reverse(list[l-n : l])
	return nil
}

// GetAppCallNoArgsFromContext parses a contract call (which is effectively an
// [opcode.SYSCALL] instruction with [interopnames.SystemContractCall]
// parameter) assuming the arguments of the contract call (usually followed by
// [opcode.PACK] opcode) are already parsed. It returns the calling contract
// hash, method name and callflags.
func GetAppCallNoArgsFromContext(ctx *Context) (util.Uint160, string, callflag.CallFlag, error) {
	instr, param, err := ctx.Next()
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse callflag instruction: %w", err)
	}
	f, err := GetInt64FromInstr(Instruction{Op: instr, Param: param})
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse callflag: %w", err)
	}
	if !callflag.IsValid(callflag.CallFlag(f)) {
		return util.Uint160{}, "", 0, fmt.Errorf("invalid callflag value: %d", f)
	}
	instr, param, err = ctx.Next()
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse method instruction: %w", err)
	}
	method, err := GetUTF8StringFromInstr(Instruction{Op: instr, Param: param})
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse method: %w", err)
	}
	if len(method) == 0 {
		return util.Uint160{}, "", 0, fmt.Errorf("empty method name")
	}
	instr, param, err = ctx.Next()
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse contract scripthash instruction: %w", err)
	}
	h, err := GetUint160FromInstr(Instruction{Op: instr, Param: param})
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse contract scripthash: %w", err)
	}
	instr, param, err = ctx.Next()
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse SYSCALL instruction: %w", err)
	}
	err = verifySystemContractCall(instr, param)
	if err != nil {
		return util.Uint160{}, "", 0, fmt.Errorf("failed to parse System.Contract.Call: %w", err)
	}
	return h, method, callflag.CallFlag(f), nil
}

// verifySystemContractCall returns an error if the provided instruction is not a
// valid System.Contract.Call SYSCALL.
func verifySystemContractCall(instr opcode.Opcode, param []byte) error {
	if instr != opcode.SYSCALL {
		return fmt.Errorf("expected SYSCALL, got %s", instr)
	}
	if len(param) != 4 {
		return fmt.Errorf("expected 4 bytes SYSCALL parameter, got %d", len(param))
	}
	name, err := interopnames.FromID(binary.LittleEndian.Uint32(param))
	if err != nil {
		return fmt.Errorf("failed to parse SYSCALL parameter: %w", err)
	}
	if name != interopnames.SystemContractCall {
		return fmt.Errorf("expected System.Contract.Call SYSCALL, got %s", name)
	}
	return nil
}

// GetAppCallFromContext parses a contract call (which is effectively an
// [opcode.SYSCALL] instruction with [interopnames.SystemContractCall]
// parameter) along with the set of arguments. It returns the calling contract
// hash, method name, callflags and arguments. It stops parsing after the first
// System.Contract.Call occurrence, so check context's IP against the script
// length if you need to ensure there are no additional opcodes after SYSCALL.
func GetAppCallFromContext(ctx *Context) (util.Uint160, string, callflag.CallFlag, []PushedItem, error) {
	return getAppCallFromContext(ctx, true)
}

// GetAppCallFromContextNonStrict is similar to GetAppCallFromContext except
// that it does not check PACK/PACKSTRUCT/PACKMAP arguments against the stack
// length. Missing elements (if any) will be filled with empty PushedItem.
func GetAppCallFromContextNonStrict(ctx *Context) (util.Uint160, string, callflag.CallFlag, []PushedItem, error) {
	return getAppCallFromContext(ctx, false)
}

func getAppCallFromContext(ctx *Context, strict bool) (util.Uint160, string, callflag.CallFlag, []PushedItem, error) {
	args, err := getListFromContext(ctx, strict)
	if err != nil {
		return util.Uint160{}, "", 0, nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	h, m, f, err := GetAppCallNoArgsFromContext(ctx)
	if err != nil {
		return util.Uint160{}, "", 0, nil, fmt.Errorf("failed to parse AppCall without args: %w", err)
	}
	return h, m, f, args, nil
}

// ParseAppCall works similar to GetAppCallFromContext except that it creates
// the parsing context by itself and ensures that there are no additional
// instructions after System.Contract.Call occurrence.
func ParseAppCall(script []byte) (util.Uint160, string, callflag.CallFlag, []PushedItem, error) {
	return parseAppCall(script, true, false, false)
}

// ParseAppCallNonStrict is similar to ParseAppCall except that it does not
// check PACK/PACKSTRUCT/PACKMAP arguments against the stack length; missing
// elements (if any) will be filled with empty PushedItem.
func ParseAppCallNonStrict(script []byte) (util.Uint160, string, callflag.CallFlag, []PushedItem, error) {
	return parseAppCall(script, false, false, false)
}

// ParseAppCallWithASSERT is similar to ParseAppCall except that it tolerates
// an optional (or required, if assertRequired is set to true) ASSERT
// instruction at the end of the script.
func ParseAppCallWithASSERT(script []byte, assertRequired bool) (util.Uint160, string, callflag.CallFlag, []PushedItem, error) {
	return parseAppCall(script, false, true, assertRequired)
}

func parseAppCall(script []byte, strict bool, assertAllowed bool, assertRequired bool) (util.Uint160, string, callflag.CallFlag, []PushedItem, error) {
	ctx := NewContext(script, 0)
	h, m, f, args, err := getAppCallFromContext(ctx, strict)
	if err != nil {
		return util.Uint160{}, "", 0, nil, fmt.Errorf("failed to parse AppCall: %w", err)
	}
	if assertAllowed {
		// Optional ASSERT is allowed.
		if ctx.NextIP() < len(script) {
			instr, _, err := ctx.Next()
			if err != nil {
				return util.Uint160{}, "", 0, nil, fmt.Errorf("%s/%s/%d: failed to parse optional ASSERT: %w", h.StringLE(), m, len(args), err)
			}
			if instr != opcode.ASSERT {
				return util.Uint160{}, "", 0, nil, fmt.Errorf("%s/%s/%d: expected ASSERT, got %s", h.StringLE(), m, len(args), instr)
			}
		} else if assertRequired {
			return util.Uint160{}, "", 0, nil, fmt.Errorf("%s/%s/%d: ASSERT required at the end of the script", h.StringLE(), m, len(args))
		}
	} /*
		if err = EnsureScriptEnd(ctx); err != nil {
			return util.Uint160{}, "", 0, nil, fmt.Errorf("%s/%s/%d: %w", h.StringLE(), m, len(args), err)
		}*/
	return h, m, f, args, nil
}

// ParseSomething works similar to GetListFromContext except that it creates
// the parsing context by itself, does not perform list unwrapping in the end,
// ensures there are no additional instructions after the first RET occurrence
// and returns an error on System.Contract.Call occurrence. If strict is set to
// false, it will not check any PACK/PACKSTRUCT/PACKMAP arguments against the
// stack length; missing elements (if any) will be filled with empty PushedItem.
func ParseSomething(script []byte, strict bool) ([]PushedItem, error) {
	ctx := NewContext(script, 0)
	res, _, err := getSomethingFromContext(ctx, strict, func(res []PushedItem) ([]PushedItem, error) {
		return nil, fmt.Errorf("SYSCALL is not allowed")
	})
	if err = EnsureScriptEnd(ctx); err != nil {
		return nil, err
	}
	return res, err
}

// ParseNEP11Transfer works similar to ParseNEP11TransferWithASSERT except that
// it doesn't require ASSERT at the end of the script.
func ParseNEP11Transfer(script []byte) (util.Uint160, util.Uint160, []byte, PushedItem, error) {
	return parseNEP11Transfer(script, false)
}

// ParseNEP11TransferWithASSERT parses a standard NEP-11 transfer script with
// ASSERT instruction at the end. It ensures no additional instructions are
// present after ASSERT. It returns the contract hash, receiver address, token
// ID and additional transfer data (may contain PUSHNULL if not provided).
func ParseNEP11TransferWithASSERT(script []byte) (util.Uint160, util.Uint160, []byte, PushedItem, error) {
	return parseNEP11Transfer(script, true)
}

func parseNEP11Transfer(script []byte, assert bool) (util.Uint160, util.Uint160, []byte, PushedItem, error) {
	ctx := NewContext(script, 0)
	h, to, tokenID, data, err := GetNEP11TransferFromContext(ctx)
	if err != nil {
		return h, to, tokenID, data, fmt.Errorf("failed to parse NEP-11 transfer: %w", err)
	}
	if assert {
		if err = GetASSERTFromContext(ctx); err != nil {
			return h, to, tokenID, data, err
		}
	}
	if err = EnsureScriptEnd(ctx); err != nil {
		return h, to, tokenID, data, err
	}
	return h, to, tokenID, data, nil
}

// GetNEP11TransferFromContext parses a standard NEP-11 transfer script. It
// returns the contract hash, receiver address, token ID and additional transfer
// data (may contain PUSHNULL if not provided). Use GetASSERTFromContext if you
// need to check if ASSERT instruction is present at the end of the script. Use
// EnsureScriptEnd if you need to ensure there are no additional instructions
// at the end of the script.
func GetNEP11TransferFromContext(ctx *Context) (util.Uint160, util.Uint160, []byte, PushedItem, error) {
	var (
		h, to   util.Uint160
		tokenID []byte
		data    PushedItem
	)
	h, m, _, args, err := GetAppCallFromContext(ctx)
	if err != nil {
		return h, to, tokenID, data, err
	}
	if m != "transfer" {
		return h, to, tokenID, data, fmt.Errorf("expected `transfer` call, got %s", m)
	}
	if len(args) != 3 {
		return h, to, tokenID, data, fmt.Errorf("expected 3 arguments, got %d", len(args))
	}
	to, err = GetUint160FromInstr(args[0].Instruction)
	if err != nil {
		return h, to, tokenID, data, fmt.Errorf("failed to parse `to` argument: %w", err)
	}
	tokenID, err = GetBytesFromInstr(args[1].Instruction)
	if err != nil {
		return h, to, tokenID, data, fmt.Errorf("failed to parse `tokenID` argument: %w", err)
	}
	return h, to, tokenID, args[2], nil
}

// ParseNEP11TransferD works similar to ParseNEP11TransferDWithASSERT except
// that it doesn't require ASSERT at the end of the script.
func ParseNEP11TransferD(script []byte) (util.Uint160, util.Uint160, util.Uint160, *big.Int, []byte, PushedItem, error) {
	return parseNEP11TransferD(script, true)
}

// ParseNEP11TransferDWithASSERT parses a standard NEP-11 transfer script with
// ASSERT instruction at the end. It ensures no additional instructions are
// present after ASSERT. It returns the contract hash, receiver address, token
// ID and additional transfer data (may contain PUSHNULL if not provided).
func ParseNEP11TransferDWithASSERT(script []byte) (util.Uint160, util.Uint160, util.Uint160, *big.Int, []byte, PushedItem, error) {
	return parseNEP11TransferD(script, true)
}

func parseNEP11TransferD(script []byte, assert bool) (util.Uint160, util.Uint160, util.Uint160, *big.Int, []byte, PushedItem, error) {
	ctx := NewContext(script, 0)
	h, from, to, amount, tokenID, data, err := GetNEP11TransferDFromContext(ctx)
	if err != nil {
		return h, from, to, amount, tokenID, data, fmt.Errorf("failed to parse NEP-11 transfer: %w", err)
	}
	if assert {
		if err = GetASSERTFromContext(ctx); err != nil {
			return h, from, to, amount, tokenID, data, err
		}
	}
	if err = EnsureScriptEnd(ctx); err != nil {
		return h, from, to, amount, tokenID, data, err
	}
	return h, from, to, amount, tokenID, data, nil
}

// GetNEP11TransferDFromContext parses a standard NEP-11 transfer script. It
// returns the contract hash, sender and receiver addresses, the amount of token
// transferred, token ID and additional transfer data (may contain PUSHNULL if
// not provided). Use GetASSERTFromContext if you need to check if ASSERT
// instruction is present at the end of the script. Use EnsureScriptEnd if you
// need to ensure there are no additional instructions after the
// System.Contract.Call.
func GetNEP11TransferDFromContext(ctx *Context) (util.Uint160, util.Uint160, util.Uint160, *big.Int, []byte, PushedItem, error) {
	var (
		h, from, to util.Uint160
		amount      *big.Int
		tokenID     []byte
		data        PushedItem
	)
	h, m, _, args, err := GetAppCallFromContext(ctx)
	if err != nil {
		return h, from, to, amount, tokenID, data, err
	}
	if m != "transfer" {
		return h, from, to, amount, tokenID, data, fmt.Errorf("expected `transfer` call, got %s", m)
	}
	if len(args) != 5 {
		return h, from, to, amount, tokenID, data, fmt.Errorf("expected 5 arguments, got %d", len(args))
	}

	from, err = GetUint160FromInstr(args[0].Instruction)
	if err != nil {
		return h, from, to, amount, tokenID, data, fmt.Errorf("failed to parse `from` argument: %w", err)
	}
	to, err = GetUint160FromInstr(args[1].Instruction)
	if err != nil {
		return h, from, to, amount, tokenID, data, fmt.Errorf("failed to parse `to` argument: %w", err)
	}
	amount, err = GetBigIntFromInstr(args[2].Instruction)
	if err != nil {
		return h, from, to, amount, tokenID, data, fmt.Errorf("failed to parse `amount` argument: %w", err)
	}
	tokenID, err = GetBytesFromInstr(args[3].Instruction)
	if err != nil {
		return h, from, to, amount, tokenID, data, fmt.Errorf("failed to parse `tokenID` argument: %w", err)
	}
	return h, from, to, amount, tokenID, args[4], nil
}

// ParseNEP17Transfer works similar to ParseNEP17TransferWithASSERT except that
// it doesn't require ASSERT at the end of the script.
func ParseNEP17Transfer(script []byte) (util.Uint160, util.Uint160, util.Uint160, *big.Int, PushedItem, error) {
	return parseNEP17Transfer(script, false)
}

// ParseNEP17TransferWithASSERT parses a standard NEP-17 transfer script with
// ASSERT instruction at the end. It ensures no additional instructions are
// present after ASSERT. It returns the contract hash, sender and receiver
// addresses, amount of transferred funds and additional transfer data (may
// contain PUSHNULL if not provided).
func ParseNEP17TransferWithASSERT(script []byte) (util.Uint160, util.Uint160, util.Uint160, *big.Int, PushedItem, error) {
	return parseNEP17Transfer(script, true)
}

func parseNEP17Transfer(script []byte, assert bool) (util.Uint160, util.Uint160, util.Uint160, *big.Int, PushedItem, error) {
	ctx := NewContext(script, 0)
	h, from, to, amount, data, err := GetNEP17TransferFromContext(ctx)
	if err != nil {
		return h, from, to, amount, data, fmt.Errorf("failed to parse NEP-17 transfer: %w", err)
	}
	if assert {
		if err = GetASSERTFromContext(ctx); err != nil {
			return h, from, to, amount, data, err
		}
	}
	if err = EnsureScriptEnd(ctx); err != nil {
		return h, from, to, amount, data, err
	}
	return h, from, to, amount, data, nil
}

// GetNEP17TransferFromContext parses a standard NEP-17 transfer script. It
// returns the contract hash, sender and receiver addresses, an amount of
// transferred token and additional transfer data (may contain PUSHNULL if not
// provided). Use GetASSERTFromContext if you need to check if ASSERT
// instruction is present at the end of the script. Use EnsureScriptEnd if you
// need to ensure there are no additional instructions at the end of the script.
func GetNEP17TransferFromContext(ctx *Context) (util.Uint160, util.Uint160, util.Uint160, *big.Int, PushedItem, error) {
	var (
		h, from, to util.Uint160
		amount      *big.Int
		data        PushedItem
	)
	h, m, _, args, err := GetAppCallFromContext(ctx)
	if err != nil {
		return h, from, to, amount, data, err
	}
	if m != "transfer" {
		return h, from, to, amount, data, fmt.Errorf("expected `transfer` call, got %s", m)
	}
	if len(args) != 4 {
		return h, from, to, amount, data, fmt.Errorf("expected 4 arguments, got %d", len(args))
	}
	from, err = GetUint160FromInstr(args[0].Instruction)
	if err != nil {
		return h, from, to, amount, data, fmt.Errorf("failed to parse `from` argument: %w", err)
	}
	to, err = GetUint160FromInstr(args[1].Instruction)
	if err != nil {
		return h, from, to, amount, data, fmt.Errorf("failed to parse `to` argument: %w", err)
	}
	amount, err = GetBigIntFromInstr(args[2].Instruction)
	if err != nil {
		return h, from, to, amount, data, fmt.Errorf("failed to parse `amount` argument: %w", err)
	}
	return h, from, to, amount, args[3], nil
}

// GetASSERTFromContext parses ASSERT instruction and returns an error if it's
// not present or if there are no more instructions.
func GetASSERTFromContext(ctx *Context) error {
	instr, _, err := ctx.Next()
	if err != nil {
		return fmt.Errorf("failed to parse ASSERT instruction: %w", err)
	}
	if instr != opcode.ASSERT {
		return fmt.Errorf("expected ASSERT, got %s", instr)
	}
	return nil
}

// EnsureScriptEnd returns an error if there are any additional instructions
// after the current context IP.
func EnsureScriptEnd(ctx *Context) error {
	if ctx.NextIP() < len(ctx.prog) {
		return fmt.Errorf("extra data after script end: expected len %d, got %d", ctx.IP(), len(ctx.prog))
	}
	return nil
}
