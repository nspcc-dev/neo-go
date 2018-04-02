package vm

import (
	"encoding/json"
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
		return &BigIntegerItem{
			value: big.NewInt(int64(val)),
		}
	case []byte:
		return &ByteArrayItem{
			value: val,
		}
	case bool:
		return &BoolItem{
			value: val,
		}
	case []StackItem:
		return &ArrayItem{
			value: val,
		}
	case *big.Int:
		return &BigIntegerItem{
			value: val,
		}
	case StackItem:
		return val
	default:
		panic(
			fmt.Sprintf(
				"invalid stack item type: %v (%v)",
				val,
				reflect.TypeOf(val),
			),
		)
	}
}

// StructItem represents a struct on the stack.
type StructItem struct {
	value []StackItem
}

// NewStructItem returns an new StructItem object.
func NewStructItem(items []StackItem) *StructItem {
	return &StructItem{
		value: items,
	}
}

// Value implements StackItem interface.
func (i *StructItem) Value() interface{} {
	return i.value
}

func (i *StructItem) String() string {
	return "Struct"
}

// BigIntegerItem represents a big integer on the stack.
type BigIntegerItem struct {
	value *big.Int
}

// NewBigIntegerItem returns an new BigIntegerItem object.
func NewBigIntegerItem(value int) *BigIntegerItem {
	return &BigIntegerItem{
		value: big.NewInt(int64(value)),
	}
}

// Value implements StackItem interface.
func (i *BigIntegerItem) Value() interface{} {
	return i.value
}

func (i *BigIntegerItem) String() string {
	return "BigInteger"
}

// MarshalJSON implements the json.Marshaler interface.
func (i *BigIntegerItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

type BoolItem struct {
	value bool
}

// NewBoolItem returns an new BoolItem object.
func NewBoolItem(val bool) *BoolItem {
	return &BoolItem{
		value: val,
	}
}

// Value implements StackItem interface.
func (i *BoolItem) Value() interface{} {
	return i.value
}

// MarshalJSON implements the json.Marshaler interface.
func (i *BoolItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

func (i *BoolItem) String() string {
	return "Bool"
}

// ByteArrayItem represents a byte array on the stack.
type ByteArrayItem struct {
	value []byte
}

// NewByteArrayItem returns an new ByteArrayItem object.
func NewByteArrayItem(b []byte) *ByteArrayItem {
	return &ByteArrayItem{
		value: b,
	}
}

// Value implements StackItem interface.
func (i *ByteArrayItem) Value() interface{} {
	return i.value
}

// MarshalJSON implements the json.Marshaler interface.
func (i *ByteArrayItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(i.value))
}

func (i *ByteArrayItem) String() string {
	return "ByteArray"
}

// ArrayItem represents a new ArrayItem object.
type ArrayItem struct {
	value []StackItem
}

// NewArrayItem returns a new ArrayItem object.
func NewArrayItem(items []StackItem) *ArrayItem {
	return &ArrayItem{
		value: items,
	}
}

// Value implements StackItem interface.
func (i *ArrayItem) Value() interface{} {
	return i.value
}

// MarshalJSON implements the json.Marshaler interface.
func (i *ArrayItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

func (i *ArrayItem) String() string {
	return "Array"
}
