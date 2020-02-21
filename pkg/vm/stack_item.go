package vm

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"

	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/vm/emit"
)

// A StackItem represents the "real" value that is pushed on the stack.
type StackItem interface {
	fmt.Stringer
	Value() interface{}
	// Dup duplicates current StackItem.
	Dup() StackItem
	// ToContractParameter converts StackItem to smartcontract.Parameter
	ToContractParameter() smartcontract.Parameter
}

func makeStackItem(v interface{}) StackItem {
	switch val := v.(type) {
	case int:
		return &BigIntegerItem{
			value: big.NewInt(int64(val)),
		}
	case int64:
		return &BigIntegerItem{
			value: big.NewInt(val),
		}
	case uint8:
		return &BigIntegerItem{
			value: big.NewInt(int64(val)),
		}
	case uint16:
		return &BigIntegerItem{
			value: big.NewInt(int64(val)),
		}
	case uint32:
		return &BigIntegerItem{
			value: big.NewInt(int64(val)),
		}
	case uint64:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, val)
		bigInt := big.NewInt(0)
		bigInt.SetBytes(b)
		return &BigIntegerItem{
			value: bigInt,
		}
	case []byte:
		return &ByteArrayItem{
			value: val,
		}
	case string:
		return &ByteArrayItem{
			value: []byte(val),
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
		var a []StackItem
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

// Dup implements StackItem interface.
func (i *StructItem) Dup() StackItem {
	// it's a reference type, so no copying here.
	return i
}

// ToContractParameter implements StackItem interface.
func (i *StructItem) ToContractParameter() smartcontract.Parameter {
	var value []smartcontract.Parameter
	for _, stackItem := range i.value {
		parameter := stackItem.ToContractParameter()
		value = append(value, parameter)
	}
	return smartcontract.Parameter{
		Type:  smartcontract.ArrayType,
		Value: value,
	}
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

// Bytes converts i to a slice of bytes.
func (i *BigIntegerItem) Bytes() []byte {
	return emit.IntToBytes(i.value)
}

// Value implements StackItem interface.
func (i *BigIntegerItem) Value() interface{} {
	return i.value
}

func (i *BigIntegerItem) String() string {
	return "BigInteger"
}

// Dup implements StackItem interface.
func (i *BigIntegerItem) Dup() StackItem {
	n := new(big.Int)
	return &BigIntegerItem{n.Set(i.value)}
}

// ToContractParameter implements StackItem interface.
func (i *BigIntegerItem) ToContractParameter() smartcontract.Parameter {
	return smartcontract.Parameter{
		Type:  smartcontract.IntegerType,
		Value: i.value.Int64(),
	}
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

// Dup implements StackItem interface.
func (i *BoolItem) Dup() StackItem {
	return &BoolItem{i.value}
}

// ToContractParameter implements StackItem interface.
func (i *BoolItem) ToContractParameter() smartcontract.Parameter {
	return smartcontract.Parameter{
		Type:  smartcontract.BoolType,
		Value: i.value,
	}
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
	return json.Marshal(hex.EncodeToString(i.value))
}

func (i *ByteArrayItem) String() string {
	return "ByteArray"
}

// Dup implements StackItem interface.
func (i *ByteArrayItem) Dup() StackItem {
	a := make([]byte, len(i.value))
	copy(a, i.value)
	return &ByteArrayItem{a}
}

// ToContractParameter implements StackItem interface.
func (i *ByteArrayItem) ToContractParameter() smartcontract.Parameter {
	return smartcontract.Parameter{
		Type:  smartcontract.ByteArrayType,
		Value: i.value,
	}
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

// Dup implements StackItem interface.
func (i *ArrayItem) Dup() StackItem {
	// reference type
	return i
}

// ToContractParameter implements StackItem interface.
func (i *ArrayItem) ToContractParameter() smartcontract.Parameter {
	var value []smartcontract.Parameter
	for _, stackItem := range i.value {
		parameter := stackItem.ToContractParameter()
		value = append(value, parameter)
	}
	return smartcontract.Parameter{
		Type:  smartcontract.ArrayType,
		Value: value,
	}
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

func (i *MapItem) String() string {
	return "Map"
}

// Has checks if map has specified key.
func (i *MapItem) Has(key StackItem) (ok bool) {
	_, ok = i.value[toMapKey(key)]
	return
}

// Dup implements StackItem interface.
func (i *MapItem) Dup() StackItem {
	// reference type
	return i
}

// ToContractParameter implements StackItem interface.
func (i *MapItem) ToContractParameter() smartcontract.Parameter {
	value := make(map[smartcontract.Parameter]smartcontract.Parameter)
	for key, val := range i.value {
		pValue := val.ToContractParameter()
		pKey := fromMapKey(key).ToContractParameter()
		if pKey.Type == smartcontract.ByteArrayType {
			pKey.Value = string(pKey.Value.([]byte))
		}
		value[pKey] = pValue
	}
	return smartcontract.Parameter{
		Type:  smartcontract.MapType,
		Value: value,
	}
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

// fromMapKey converts map key to StackItem
func fromMapKey(key interface{}) StackItem {
	switch t := key.(type) {
	case bool:
		return &BoolItem{value: t}
	case int64:
		return &BigIntegerItem{value: big.NewInt(t)}
	case string:
		return &ByteArrayItem{value: []byte(t)}
	default:
		panic("wrong key type")
	}
}

// InteropItem represents interop data on the stack.
type InteropItem struct {
	value interface{}
}

// NewInteropItem returns new InteropItem object.
func NewInteropItem(value interface{}) *InteropItem {
	return &InteropItem{
		value: value,
	}
}

// Value implements StackItem interface.
func (i *InteropItem) Value() interface{} {
	return i.value
}

// String implements stringer interface.
func (i *InteropItem) String() string {
	return "InteropItem"
}

// Dup implements StackItem interface.
func (i *InteropItem) Dup() StackItem {
	// reference type
	return i
}

// ToContractParameter implements StackItem interface.
func (i *InteropItem) ToContractParameter() smartcontract.Parameter {
	return smartcontract.Parameter{
		Type:  smartcontract.InteropInterfaceType,
		Value: nil,
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (i *InteropItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}
