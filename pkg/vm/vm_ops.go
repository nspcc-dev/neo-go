package vm

import "github.com/CityOfZion/neo-go/pkg/vm/stack"

var opFunc = map[stack.Instruction]func(ctx *stack.Context, istack *stack.Invocation) error{
	stack.ADD: Add,
	stack.SUB: Sub,
}
