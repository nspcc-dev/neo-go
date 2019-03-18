package vm

import (
	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// Flow control

// RET Returns from the current context
// Returns HALT if there are nomore context's to run
func RET(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	// Pop current context from the Inovation stack
	ctx, err := istack.PopCurrentContext()
	if err != nil {
		return FAULT, err
	}
	// If this was the last context, then we copy over the evaluation stack to the resultstack
	// As the program is about to terminate, once we remove the context
	if istack.Len() == 0 {

		err = ctx.Estack.CopyTo(rstack)
		return HALT, err
	}

	return NONE, nil
}
