package vm

import "github.com/CityOfZion/neo-go/pkg/vm/stack"

var opFunc = map[stack.Instruction]func(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation) (Vmstate, error){
	stack.ADD:         Add,
	stack.SUB:         Sub,
	stack.PUSHBYTES1:  PushNBytes,
	stack.PUSHBYTES75: PushNBytes,
	stack.RET:         RET,
	stack.EQUAL:       EQUAL,
	stack.THROWIFNOT:  THROWIFNOT,
}

func init() {
	for i := int(stack.PUSHBYTES1); i <= int(stack.PUSHBYTES75); i++ {
		opFunc[stack.Instruction(i)] = PushNBytes
	}
}
