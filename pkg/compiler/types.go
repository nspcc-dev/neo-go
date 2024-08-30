package compiler

import (
	"go/ast"
	"go/types"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

func (c *codegen) typeAndValueOf(e ast.Expr) types.TypeAndValue {
	for i := len(c.pkgInfoInline) - 1; i >= 0; i-- {
		if tv, ok := c.pkgInfoInline[i].TypesInfo.Types[e]; ok {
			return tv
		}
	}

	if tv, ok := c.typeInfo.Types[e]; ok {
		return tv
	}

	se, ok := e.(*ast.SelectorExpr)
	if ok {
		if tv, ok := c.typeInfo.Selections[se]; ok {
			return types.TypeAndValue{Type: tv.Type()}
		}
	}
	return types.TypeAndValue{}
}

func (c *codegen) typeOf(e ast.Expr) types.Type {
	for i := len(c.pkgInfoInline) - 1; i >= 0; i-- {
		if typ := c.pkgInfoInline[i].TypesInfo.TypeOf(e); typ != nil {
			return typ
		}
	}
	for _, p := range c.packageCache {
		typ := p.TypesInfo.TypeOf(e)
		if typ != nil {
			return typ
		}
	}
	return nil
}

func isBasicTypeOfKind(typ types.Type, ks ...types.BasicKind) bool {
	if t, ok := typ.Underlying().(*types.Basic); ok {
		k := t.Kind()
		return slices.Contains(ks, k)
	}
	return false
}

func isMap(typ types.Type) bool {
	_, ok := typ.Underlying().(*types.Map)
	return ok
}

func isByte(typ types.Type) bool {
	return isBasicTypeOfKind(typ, types.Uint8, types.Int8)
}

func isBool(typ types.Type) bool {
	return isBasicTypeOfKind(typ, types.Bool, types.UntypedBool)
}

func isNumber(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Basic)
	return ok && t.Info()&types.IsNumeric != 0
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

func toNeoType(typ types.Type) stackitem.Type {
	if typ == nil {
		return stackitem.AnyT
	}
	switch t := typ.Underlying().(type) {
	case *types.Basic:
		info := t.Info()
		switch {
		case info&types.IsInteger != 0:
			return stackitem.IntegerT
		case info&types.IsBoolean != 0:
			return stackitem.BooleanT
		case info&types.IsString != 0:
			return stackitem.ByteArrayT
		default:
			return stackitem.AnyT
		}
	case *types.Map:
		return stackitem.MapT
	case *types.Struct:
		return stackitem.StructT
	case *types.Slice:
		if isByte(t.Elem()) {
			return stackitem.BufferT
		}
		return stackitem.ArrayT
	default:
		return stackitem.AnyT
	}
}
