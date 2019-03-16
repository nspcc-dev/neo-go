package vm

import (
	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// Flow control

// RET Returns from the current context
// Returns HALT if there are nomore context's to run
func RET(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation) (Vmstate, error) {

	// Pop current context from the Inovation stack
	err := istack.RemoveCurrentContext()
	if err != nil {
		return FAULT, err
	}

	// If there are no-more context's left to ran, then we HALT
	if istack.Len() == 0 {
		return HALT, nil
	}

	return NONE, nil
}
