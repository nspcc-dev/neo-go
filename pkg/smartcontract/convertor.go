package smartcontract

import (
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ParameterFromStackItem converts stackitem.Item to Parameter.
func ParameterFromStackItem(i stackitem.Item, seen map[stackitem.Item]bool) Parameter {
	switch t := i.(type) {
	case stackitem.Null, *stackitem.Pointer:
		return NewParameter(AnyType)
	case *stackitem.BigInteger:
		return Parameter{
			Type:  IntegerType,
			Value: i.Value().(*big.Int),
		}
	case stackitem.Bool:
		return Parameter{
			Type:  BoolType,
			Value: i.Value().(bool),
		}
	case *stackitem.ByteArray:
		return Parameter{
			Type:  ByteArrayType,
			Value: i.Value().([]byte),
		}
	case *stackitem.Interop:
		return Parameter{
			Type:  InteropInterfaceType,
			Value: nil,
		}
	case *stackitem.Buffer:
		return Parameter{
			Type:  ByteArrayType,
			Value: i.Value().([]byte),
		}
	case *stackitem.Struct, *stackitem.Array:
		var value []Parameter

		if !seen[i] {
			seen[i] = true
			for _, stackItem := range i.Value().([]stackitem.Item) {
				parameter := ParameterFromStackItem(stackItem, seen)
				value = append(value, parameter)
			}
		}
		return Parameter{
			Type:  ArrayType,
			Value: value,
		}
	case *stackitem.Map:
		value := make([]ParameterPair, 0)
		if !seen[i] {
			seen[i] = true
			for _, element := range i.Value().([]stackitem.MapElement) {
				value = append(value, ParameterPair{
					Key:   ParameterFromStackItem(element.Key, seen),
					Value: ParameterFromStackItem(element.Value, seen),
				})
			}
		}
		return Parameter{
			Type:  MapType,
			Value: value,
		}
	default:
		panic(fmt.Sprintf("unknown stack item type: %v", t))
	}
}
