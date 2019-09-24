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
	case []int:
		a := []StackItem{}
		for _, i := range val {
			a = append(a, makeStackItem(i))
		}
		return makeStackItem(a)
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

// Clone returns a Struct with all Struct fields copied by value.
// Array fields are still copied by reference.
func (i *StructItem) Clone() *StructItem {
	ret := &StructItem{make([]StackItem, len(i.value))}
	for j := range i.value {
		switch t := i.value[j].(type) {
		case *StructItem:
			ret.value[j] = t.Clone()
		default:
			ret.value[j] = t
		}
	}
	return ret
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

// BoolItem represents a boolean StackItem.
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

// MapItem represents Map object.
type MapItem struct {
	value map[interface{}]StackItem
}

// NewMapItem returns new MapItem object.
func NewMapItem() *MapItem {
	return &MapItem{
		value: make(map[interface{}]StackItem),
	}
}

// Value implements StackItem interface.
func (i *MapItem) Value() interface{} {
	return i.value
}

// MarshalJSON implements the json.Marshaler interface.
func (i *MapItem) String() string {
	return "Map"
}

// Has checks if map has specified key.
func (i *MapItem) Has(key StackItem) (ok bool) {
	_, ok = i.value[toMapKey(key)]
	return
}

// Add adds key-value pair to the map.
func (i *MapItem) Add(key, value StackItem) {
	i.value[toMapKey(key)] = value
}

// toMapKey converts StackItem so that it can be used as a map key.
func toMapKey(key StackItem) interface{} {
	switch t := key.(type) {
	case *BoolItem:
		return t.value
	case *BigIntegerItem:
		return t.value.Int64()
	case *ByteArrayItem:
		return string(t.value)
	default:
		panic("wrong key type")
	}
}
