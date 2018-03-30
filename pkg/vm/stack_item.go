package vm

import (
	"fmt"
	"math/big"
	"reflect"
)

// A StackItem represents the "real" value that is pushed on the stack.
type StackItem interface {
	fmt.Stringer
	Value() interface{}
}

func makeStackItem(v interface{}) StackItem {
	switch val := v.(type) {
	case int:
		return &bigIntegerItem{
			value: big.NewInt(int64(val)),
		}
	case []byte:
		return &byteArrayItem{
			value: val,
		}
	case bool:
		return &boolItem{
			value: val,
		}
	case []StackItem:
		return &arrayItem{
			value: val,
		}
	case *big.Int:
		return &bigIntegerItem{
			value: val,
		}
	case StackItem:
		return val
	default:
		panic(
			fmt.Sprintf(
				"invalid stack item type: %v (%s)",
				val,
				reflect.TypeOf(val),
			),
		)
	}
}

type structItem struct {
	value []StackItem
}

// Value implements StackItem interface.
func (i *structItem) Value() interface{} {
	return i.value
}

func (i *structItem) String() string {
	return "Struct"
}

type bigIntegerItem struct {
	value *big.Int
}

// Value implements StackItem interface.
func (i *bigIntegerItem) Value() interface{} {
	return i.value
}

func (i *bigIntegerItem) String() string {
	return "BigInteger"
}

type boolItem struct {
	value bool
}

// Value implements StackItem interface.
func (i *boolItem) Value() interface{} {
	return i.value
}

func (i *boolItem) String() string {
	return "Bool"
}

type byteArrayItem struct {
	value []byte
}

// Value implements StackItem interface.
func (i *byteArrayItem) Value() interface{} {
	return i.value
}

func (i *byteArrayItem) String() string {
	return "ByteArray"
}

type arrayItem struct {
	value []StackItem
}

// Value implements StackItem interface.
func (i *arrayItem) Value() interface{} {
	return i.value
}

func (i *arrayItem) String() string {
	return "Array"
}
