package compiler

import (
	"go/ast"
	"go/types"
	"log"

	"github.com/CityOfZion/neo-go/pkg/vm"
)

type jumpLabel struct {
	offset int
	op     vm.OpCode
}

// A FuncContext represents details about a function in the program along withs its variables.
type FuncContext struct {
	// The declaration tree of this function.
	decl *ast.FuncDecl
	// Identifier (name of the function in the program).
	name string
	// The scope of the function.
	scope map[string]*VarContext
	// Arguments of the function.
	args map[string]bool
	// Address (label) where the compiler can find this function when someone calls it.
	label int16
	// Counter for stored local variables.
	i int
	// This needs refactor along with the (if stmt)
	jumpLabels []jumpLabel
}

func (f *FuncContext) addJump(op vm.OpCode, offset int) {
	f.jumpLabels = append(f.jumpLabels, jumpLabel{offset, op})
}

func newFuncContext(decl *ast.FuncDecl, label int16) *FuncContext {
	return &FuncContext{
		decl:       decl,
		label:      int16(label),
		name:       decl.Name.Name,
		scope:      map[string]*VarContext{},
		args:       map[string]bool{},
		jumpLabels: []jumpLabel{},
	}
}

func (f *FuncContext) newConst(name string, t types.TypeAndValue, needStore bool) *VarContext {
	ctx := &VarContext{
		name:  name,
		tinfo: t,
	}
	if needStore {
		f.storeContext(ctx)
	}
	return ctx
}

func (f *FuncContext) numStackOps() int64 {
	ops := 0
	ast.Inspect(f.decl, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.AssignStmt, *ast.ReturnStmt, *ast.IfStmt:
			ops++
		}
		return true
	})

	numArgs := len(f.decl.Type.Params.List)
	return int64(ops + numArgs)
}

func (f *FuncContext) storeContext(ctx *VarContext) {
	ctx.pos = f.i
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
