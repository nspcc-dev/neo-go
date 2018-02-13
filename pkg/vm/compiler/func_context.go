package compiler

import (
	"log"
)

// A FuncContext represents details about a function in the program along withs its variables.
type FuncContext struct {
	// Identifier.
	name string
	// The scope of the function.
	scope map[string]*VarContext
	// Arguments of the function.
	args map[string]bool
	// Address (label) where the compiler can find this function when someone calls it.
	label int16
	// Counter for local variables.
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

func (f *FuncContext) registerContext(ctx *VarContext, setPos bool) {
	if setPos {
		ctx.pos = f.i
	}
	f.scope[ctx.name] = ctx
	f.i++
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
