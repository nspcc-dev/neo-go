package newcompiler

import (
	"go/ast"
	"go/types"
)

// convertCompositeLit will return true if the LHS of the assign needs to be stored locally.
// Some literal composites, like "struct literals", will handle those local variables in a different way.
func (c *codegen) convertCompositeLit(clit *ast.CompositeLit) bool {
	switch t := clit.Type.(type) {
	case *ast.Ident:
		typ := c.typeInfo.ObjectOf(t).Type().Underlying()
		switch typ.(type) {
		case *types.Struct:
			emitOpcode(c.prog, Onop)
			emitInt(c.prog, int64(len(clit.Elts)))
			emitOpcode(c.prog, Onewstruct)
			emitOpcode(c.prog, Otoaltstack)

			for _, field := range clit.Elts {
				f := field.(*ast.KeyValueExpr)
				l := c.newLocal(f.Key.(*ast.Ident).Name)
				c.emitLoadConst(c.getTypeInfo(f.Value))
				c.emitStoreLocal(l)
			}
			return false
		}

	// default converts inline composite literals like:
	// []int{}, []string{}
	// TODO: converting lits with custom types:
	// []foo{myfoo, yourfoo, everyonesfoo}
	default:
		n := len(clit.Elts)
		for i := n - 1; i >= 0; i-- {
			c.emitLoadConst(c.getTypeInfo(clit.Elts[i]))
		}
		emitInt(c.prog, int64(n))
		emitOpcode(c.prog, Opack)
	}
	return true
}
