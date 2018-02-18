package newcompiler

import (
	"go/ast"
)

func (c *codegen) convertStruct(lit *ast.CompositeLit, lhs ast.Expr) {
	emitOpcode(c.prog, Onop)
	emitInt(c.prog, int64(len(lit.Elts)))
	emitOpcode(c.prog, Onewstruct)
	emitOpcode(c.prog, Otoaltstack)

	// Create a new struct scope to store the positions of its variables.
	strct := newStructScope()

	for _, field := range lit.Elts {
		f := field.(*ast.KeyValueExpr)
		// Walk to resolve the expression of the value.
		ast.Walk(c, f.Value)
		l := strct.newField(f.Key.(*ast.Ident).Name)
		c.emitStoreLocal(l)
	}

	emitOpcode(c.prog, Ofromaltstack)

	switch t := lhs.(type) {
	case *ast.Ident:
		c.fctx.structs[t.Name] = strct
		l := c.fctx.loadVar(t.Name)
		c.emitStoreLocal(l)
	case *ast.SelectorExpr:
	}
}

// convertCompositeLit will return true if the LHS of the assign needs to be stored locally.
// Some literal composites, like "struct literals", will handle those local variables in a different way.
func (c *codegen) convertCompositeLit(clit *ast.CompositeLit, lhs *ast.Ident) {

}
