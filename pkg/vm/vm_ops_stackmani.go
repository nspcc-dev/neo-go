package vm

import (
	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// Stack Manipulation Opcodes

// PushNBytes will Read N Bytes from the script and push it onto the stack
func PushNBytes(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation) error {

	val, err := ctx.ReadBytes(int(op))
	if err != nil {
		return err
	}
	ba := stack.NewByteArray(val)
	ctx.Estack.Push(ba)
	return nil
}
