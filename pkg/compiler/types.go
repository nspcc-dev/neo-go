package compiler

import (
	"go/ast"
	"go/types"
)

func (c *codegen) typeAndValueOf(e ast.Expr) types.TypeAndValue {
	return c.typeInfo.Types[e]
}

func (c *codegen) typeOf(e ast.Expr) types.Type {
	return c.typeAndValueOf(e).Type
}

func isBasicTypeOfKind(typ types.Type, ks ...types.BasicKind) bool {
	if t, ok := typ.Underlying().(*types.Basic); ok {
		k := t.Kind()
		for i := range ks {
			if k == ks[i] {
				return true
			}
		}
	}
	return false
}

func isByte(typ types.Type) bool {
	return isBasicTypeOfKind(typ, types.Uint8, types.Int8)
}

func isString(typ types.Type) bool {
	return isBasicTypeOfKind(typ, types.String)
}

func isCompoundSlice(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Slice)
	return ok && !isByte(t.Elem())
}

func isByteSlice(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Slice)
	return ok && isByte(t.Elem())
}
