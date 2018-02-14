package compiler

import (
	"go/types"
	"log"
)

// A FuncContext represents details about a function in the program along withs its variables.
type FuncContext struct {
	// Identifier (name of the function in the program).
	name string
	// The scope of the function.
	scope map[string]*VarContext
	// Arguments of the function.
	args map[string]bool
	// Address (label) where the compiler can find this function when someone calls it.
	label int16

	// Total size of the stack this function will need.
	// arguments + stored locals + all consts
	stackSize int
	// Counter for stored local variables.
	i int
}

func newFuncContext(name string, label int16) *FuncContext {
	return &FuncContext{
		label: int16(label),
		name:  name,
		scope: map[string]*VarContext{},
		args:  map[string]bool{},
	}
}

func (f *FuncContext) newConst(name string, t types.TypeAndValue, register, setPos bool) *VarContext {
	f.stackSize++

	ctx := &VarContext{
		name:  name,
		tinfo: t,
	}
	if register {
		f.registerContext(ctx, setPos)
	}

	return ctx
}

func (f *FuncContext) registerContext(ctx *VarContext, setPos bool) {
	if setPos {
		ctx.pos = f.i
		f.i++
	}
	f.scope[ctx.name] = ctx
}

func (f *FuncContext) getContext(name string) *VarContext {
	ctx, ok := f.scope[name]
	if !ok {
		log.Fatalf("could not resolve variable %s", name)
	}
	return ctx
}

func (f *FuncContext) isRegistered(ctx *VarContext) bool {
	_, ok := f.scope[ctx.name]
	return ok
}

func (f *FuncContext) isArgument(name string) bool {
	_, ok := f.args[name]
	return ok
}
