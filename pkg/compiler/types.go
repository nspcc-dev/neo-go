package compiler

import (
	"go/ast"
	"go/types"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
