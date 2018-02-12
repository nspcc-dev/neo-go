package compiler

import (
	"go/ast"
	"go/types"
	"log"
)

// A FuncContext represents details about a function in the program, along withs its variables.
type FuncContext struct {
	name string
	vars map[string]*VarContext
	i    int
}

func newFuncContext(name string) *FuncContext {
	return &FuncContext{
		name: name,
		vars: map[string]*VarContext{},
	}
}

func (f *FuncContext) putContext(ctx *VarContext, setPos bool) {
	if setPos {
		ctx.pos = f.i
	}
	f.vars[ctx.name] = ctx
	f.i++
}

func (f *FuncContext) getContext(name string) *VarContext {
	ctx, ok := f.vars[name]
	if !ok {
		log.Fatalf("could not resolve variable %s", name)
	}
	return ctx
}

func (f *FuncContext) varContextFromExpr(expr ast.Expr, tinfo *types.Info) *VarContext {
	var ctx *VarContext

	switch t := expr.(type) {
	case *ast.Ident:
		ctx = f.getContext(t.Name)
	case *ast.BasicLit:
		ctx = newVarContext(tinfo.Types[t])
	case *ast.BinaryExpr:
		ctx = resolveBinaryExpr(f, t, tinfo)
	}

	return ctx
}

func (f *FuncContext) isRegistered(ctx *VarContext) bool {
	_, ok := f.vars[ctx.name]
	return ok
}
