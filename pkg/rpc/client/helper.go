package client

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client/nns"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// getInvocationError returns an error in case of bad VM state or an empty stack.
func getInvocationError(result *result.Invoke) error {
	if result.State != "HALT" {
		return fmt.Errorf("invocation failed: %s", result.FaultException)
	}
	if len(result.Stack) == 0 {
		return errors.New("result stack is empty")
	}
	return nil
}

// topBoolFromStack returns the top boolean value from the stack.
func topBoolFromStack(st []stackitem.Item) (bool, error) {
	index := len(st) - 1 // top stack element is last in the array
	result, ok := st[index].Value().(bool)
	if !ok {
		return false, fmt.Errorf("invalid stack item type: %s", st[index].Type())
	}
	return result, nil
}

// topIntFromStack returns the top integer value from the stack.
func topIntFromStack(st []stackitem.Item) (int64, error) {
	index := len(st) - 1 // top stack element is last in the array
	bi, err := st[index].TryInteger()
	if err != nil {
		return 0, err
	}
	return bi.Int64(), nil
}

// topPublicKeysFromStack returns the top array of public keys from the stack.
func topPublicKeysFromStack(st []stackitem.Item) (keys.PublicKeys, error) {
	index := len(st) - 1 // top stack element is last in the array
	var (
		pks keys.PublicKeys
		err error
	)
	items, ok := st[index].Value().([]stackitem.Item)
	if !ok {
		return nil, fmt.Errorf("invalid stack item type: %s", st[index].Type())
	}
	pks = make(keys.PublicKeys, len(items))
	for i, item := range items {
		val, ok := item.Value().([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid array element #%d: %s", i, item.Type())
		}
		pks[i], err = keys.NewPublicKeyFromBytes(val, elliptic.P256())
		if err != nil {
			return nil, err
		}
	}
	return pks, nil
}

// top string from stack returns the top string from the stack.
func topStringFromStack(st []stackitem.Item) (string, error) {
	index := len(st) - 1 // top stack element is last in the array
	bs, err := st[index].TryBytes()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// topUint160FromStack returns the top util.Uint160 from the stack.
func topUint160FromStack(st []stackitem.Item) (util.Uint160, error) {
	index := len(st) - 1 // top stack element is last in the array
	bs, err := st[index].TryBytes()
	if err != nil {
		return util.Uint160{}, err
	}
	return util.Uint160DecodeBytesBE(bs)
}

// topMapFromStack returns the top stackitem.Map from the stack.
func topMapFromStack(st []stackitem.Item) (*stackitem.Map, error) {
	index := len(st) - 1 // top stack element is last in the array
	if t := st[index].Type(); t != stackitem.MapT {
		return nil, fmt.Errorf("invalid return stackitem type: %s", t.String())
	}
	return st[index].(*stackitem.Map), nil
}

// InvokeAndPackIteratorResults creates a script containing System.Contract.Call
// of the specified contract with the specified arguments. It assumes that the
// specified operation will return iterator. The script traverses the resulting
// iterator, packs all its values into array and pushes the resulting array on
// stack. Constructed script is invoked via `invokescript` JSON-RPC API using
// the provided signers. The result of the script invocation contains single array
// stackitem on stack if invocation HALTed. InvokeAndPackIteratorResults can be
// used to interact with JSON-RPC server where iterator sessions are disabled to
// retrieve iterator values via single `invokescript` JSON-RPC call.
func (c *Client) InvokeAndPackIteratorResults(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	bytes, err := createIteratorUnwrapperScript(contract, operation, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator unwrapper script: %w", err)
	}
	return c.InvokeScript(bytes, signers)
}

func createIteratorUnwrapperScript(contract util.Uint160, operation string, params []smartcontract.Parameter) ([]byte, error) {
	script := io.NewBufBinWriter()
	// Pack arguments for System.Contract.Call.
	arr, err := smartcontract.ExpandParameterToEmitable(smartcontract.Parameter{
		Type:  smartcontract.ArrayType,
		Value: params,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to expand parameters array to emitable: %w", err)
	}
	emit.Array(script.BinWriter, arr.([]interface{})...)
	emit.AppCallNoArgs(script.BinWriter, contract, operation, callflag.All) // The System.Contract.Call itself, it will push Iterator on estack.
	emit.Opcodes(script.BinWriter, opcode.NEWARRAY0)                        // Push new empty array to estack. This array will store iterator's elements.

	// Start the iterator traversal cycle.
	iteratorTraverseCycleStartOffset := script.Len()
	emit.Opcodes(script.BinWriter, opcode.OVER)                     // Load iterator from 1-st cell of estack.
	emit.Syscall(script.BinWriter, interopnames.SystemIteratorNext) // Call System.Iterator.Next, it will pop the iterator from estack and push `true` or `false` to estack.
	jmpIfNotOffset := script.Len()
	emit.Instruction(script.BinWriter, opcode.JMPIFNOT, // Pop boolean value (from the previous step) from estack, if `false`, then iterator has no more items => jump to the end of program.
		[]byte{
			0x00, // jump to loadResultOffset, but we'll fill this byte after script creation.
		})
	emit.Opcodes(script.BinWriter, opcode.DUP, // Duplicate the resulting array from 0-th cell of estack and push it to estack.
		opcode.PUSH2, opcode.PICK) // Pick iterator from the 2-nd cell of estack.
	emit.Syscall(script.BinWriter, interopnames.SystemIteratorValue) // Call System.Iterator.Value, it will pop the iterator from estack and push its current value to estack.
	emit.Opcodes(script.BinWriter, opcode.APPEND)                    // Pop iterator value and the resulting array from estack. Append value to the resulting array. Array is a reference type, thus, value stored at the 1-th cell of local slot will also be updated.
	jmpOffset := script.Len()
	emit.Instruction(script.BinWriter, opcode.JMP, // Jump to the start of iterator traverse cycle.
		[]byte{
			uint8(iteratorTraverseCycleStartOffset - jmpOffset), // jump to iteratorTraverseCycleStartOffset; offset is relative to JMP position.
		})

	// End of the program: push the result on stack and return.
	loadResultOffset := script.Len()
	emit.Opcodes(script.BinWriter, opcode.NIP) // Remove iterator from the 1-st cell of estack, so that only resulting array is left on estack.
	if err := script.Err; err != nil {
		return nil, fmt.Errorf("failed to build iterator unwrapper script: %w", err)
	}

	// Fill in JMPIFNOT instruction parameter.
	bytes := script.Bytes()
	bytes[jmpIfNotOffset+1] = uint8(loadResultOffset - jmpIfNotOffset) // +1 is for JMPIFNOT itself; offset is relative to JMPIFNOT position.
	return bytes, nil
}

// topIterableFromStack returns the list of elements of `resultItemType` type from the top element
// of the provided stack. The top element is expected to be an Array, otherwise an error is returned.
func topIterableFromStack(st []stackitem.Item, resultItemType interface{}) ([]interface{}, error) {
	index := len(st) - 1 // top stack element is the last in the array
	if t := st[index].Type(); t != stackitem.ArrayT {
		return nil, fmt.Errorf("invalid return stackitem type: %s (Array expected)", t.String())
	}
	items, ok := st[index].Value().([]stackitem.Item)
	if !ok {
		return nil, fmt.Errorf("failed to deserialize iterable from Array stackitem: invalid value type (Array expected)")
	}
	result := make([]interface{}, len(items))
	for i := range items {
		switch resultItemType.(type) {
		case []byte:
			bytes, err := items[i].TryBytes()
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize []byte from stackitem #%d: %w", i, err)
			}
			result[i] = bytes
		case string:
			bytes, err := items[i].TryBytes()
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize string from stackitem #%d: %w", i, err)
			}
			result[i] = string(bytes)
		case util.Uint160:
			bytes, err := items[i].TryBytes()
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize uint160 from stackitem #%d: %w", i, err)
			}
			result[i], err = util.Uint160DecodeBytesBE(bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to decode uint160 from stackitem #%d: %w", i, err)
			}
		case nns.RecordState:
			rs, ok := items[i].Value().([]stackitem.Item)
			if !ok {
				return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: not a struct", i)
			}
			if len(rs) != 3 {
				return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: wrong number of elements", i)
			}
			name, err := rs[0].TryBytes()
			if err != nil {
				return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: %w", i, err)
			}
			typ, err := rs[1].TryInteger()
			if err != nil {
				return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: %w", i, err)
			}
			data, err := rs[2].TryBytes()
			if err != nil {
				return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: %w", i, err)
			}
			u64Typ := typ.Uint64()
			if !typ.IsUint64() || u64Typ > 255 {
				return nil, fmt.Errorf("failed to decode RecordState from stackitem #%d: bad type", i)
			}
			result[i] = nns.RecordState{
				Name: string(name),
				Type: nns.RecordType(u64Typ),
				Data: string(data),
			}
		default:
			return nil, errors.New("unsupported iterable type")
		}
	}
	return result, nil
}
