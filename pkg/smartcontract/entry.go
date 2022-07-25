package smartcontract

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// CreateCallAndUnwrapIteratorScript creates a script that calls 'operation' method
// of the 'contract' with the specified arguments. This method is expected to return
// an iterator that then is traversed (using iterator.Next) with values (iterator.Value)
// extracted and added into array. At most maxIteratorResultItems number of items is
// processed this way (and this number can't exceed VM limits), the result of the
// script is an array containing extracted value elements. This script can be useful
// for interactions with RPC server that have iterator sessions disabled.
func CreateCallAndUnwrapIteratorScript(contract util.Uint160, operation string, params []Parameter, maxIteratorResultItems int) ([]byte, error) {
	script := io.NewBufBinWriter()
	emit.Int(script.BinWriter, int64(maxIteratorResultItems))
	// Pack arguments for System.Contract.Call.
	arr, err := ExpandParameterToEmitable(Parameter{
		Type:  ArrayType,
		Value: params,
	})
	if err != nil {
		return nil, fmt.Errorf("expanding params to emitable: %w", err)
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
	emit.Opcodes(script.BinWriter, opcode.DUP,                       // Duplicate the resulting array from 0-th cell of estack and push it to estack.
		opcode.SIZE,               // Pop array from estack and push its size to estack.
		opcode.PUSH3, opcode.PICK, // Pick maxIteratorResultItems from the 3-d cell of estack.
		opcode.GE) // Compare len(arr) and maxIteratorResultItems
	jmpIfMaxReachedOffset := script.Len()
	emit.Instruction(script.BinWriter, opcode.JMPIF, // Pop boolean value (from the previous step) from estack, if `false`, then max array elements is reached => jump to the end of program.
		[]byte{
			0x00, // jump to loadResultOffset, but we'll fill this byte after script creation.
		})
	jmpOffset := script.Len()
	emit.Instruction(script.BinWriter, opcode.JMP, // Jump to the start of iterator traverse cycle.
		[]byte{
			uint8(iteratorTraverseCycleStartOffset - jmpOffset), // jump to iteratorTraverseCycleStartOffset; offset is relative to JMP position.
		})

	// End of the program: push the result on stack and return.
	loadResultOffset := script.Len()
	emit.Opcodes(script.BinWriter, opcode.NIP, // Remove iterator from the 1-st cell of estack
		opcode.NIP) // Remove maxIteratorResultItems from the 1-st cell of estack, so that only resulting array is left on estack.
	if err := script.Err; err != nil {
		return nil, fmt.Errorf("emitting iterator unwrapper script: %w", err)
	}

	// Fill in JMPIFNOT instruction parameter.
	bytes := script.Bytes()
	bytes[jmpIfNotOffset+1] = uint8(loadResultOffset - jmpIfNotOffset) // +1 is for JMPIFNOT itself; offset is relative to JMPIFNOT position.
	// Fill in jmpIfMaxReachedOffset instruction parameter.
	bytes[jmpIfMaxReachedOffset+1] = uint8(loadResultOffset - jmpIfMaxReachedOffset) // +1 is for JMPIF itself; offset is relative to JMPIF position.
	return bytes, nil
}
