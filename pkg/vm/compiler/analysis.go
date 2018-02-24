package compiler

import (
	"go/ast"
	"go/constant"
	"go/types"
	"log"
)

// countGlobals counts the global variables in the program to add
// them with the stacksize of the function.
func countGlobals(f *ast.File) (i int64) {
	ast.Inspect(f, func(node ast.Node) bool {
		switch node.(type) {
		// Skip all functio declarations.
		case *ast.FuncDecl:
			return false
		// After skipping all funcDecls we are sure that each value spec
		// is a global declared variable or constant.
		case *ast.ValueSpec:
			i++
		}
		return true
	})
	return
}

// isIdentBool looks if the given ident is a boolean.
func isIdentBool(ident *ast.Ident) bool {
	return ident.Name == "true" || ident.Name == "false"
}

// makeBoolFromIdent creates a bool type from an *ast.Ident.
func makeBoolFromIdent(ident *ast.Ident, tinfo *types.Info) types.TypeAndValue {
	var b bool
	if ident.Name == "true" {
		b = true
	} else if ident.Name == "false" {
		b = false
	} else {
		log.Fatalf("givent identifier cannot be converted to a boolean => %s", ident.Name)
	}
	return types.TypeAndValue{
		Type:  tinfo.ObjectOf(ident).Type(),
		Value: constant.MakeBool(b),
	}
}
