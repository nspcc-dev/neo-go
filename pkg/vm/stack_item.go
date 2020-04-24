package vm

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// A StackItem represents the "real" value that is pushed on the stack.
type StackItem interface {
	fmt.Stringer
	Value() interface{}
	// Dup duplicates current StackItem.
	Dup() StackItem
	// TryBytes converts StackItem to a byte slice.
	TryBytes() ([]byte, error)
	// TryInteger converts StackItem to an integer.
	TryInteger() (*big.Int, error)
	// Equals checks if 2 StackItems are equal.
	Equals(s StackItem) bool
	// ToContractParameter converts StackItem to smartcontract.Parameter
	ToContractParameter(map[StackItem]bool) smartcontract.Parameter
	// Type returns stack item type.
	Type() StackItemType
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
		i64T := reflect.TypeOf(int64(0))
		if reflect.TypeOf(val).ConvertibleTo(i64T) {
			i64Val := reflect.ValueOf(val).Convert(i64T).Interface()
			return makeStackItem(i64Val)
		}
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

// TryBytes implements StackItem interface.
func (i *StructItem) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Struct to ByteArray")
}

// TryInteger implements StackItem interface.
func (i *StructItem) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Struct to Integer")
}

// Equals implements StackItem interface.
func (i *StructItem) Equals(s StackItem) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*StructItem)
	if !ok || len(i.value) != len(val.value) {
		return false
	}
	for j := range i.value {
		if !i.value[j].Equals(val.value[j]) {
			return false
		}
	}
	return true
}

// ToContractParameter implements StackItem interface.
func (i *StructItem) ToContractParameter(seen map[StackItem]bool) smartcontract.Parameter {
	var value []smartcontract.Parameter

	if !seen[i] {
		seen[i] = true
		for _, stackItem := range i.value {
			parameter := stackItem.ToContractParameter(seen)
			value = append(value, parameter)
		}
	}
	return smartcontract.Parameter{
		Type:  smartcontract.ArrayType,
		Value: value,
	}
}

// Type implements StackItem interface.
func (i *StructItem) Type() StackItemType { return StructT }

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

// NullItem represents null on the stack.
type NullItem struct{}

// String implements StackItem interface.
func (i NullItem) String() string {
	return "Null"
}

// Value implements StackItem interface.
func (i NullItem) Value() interface{} {
	return nil
}

// Dup implements StackItem interface.
// There is no need to perform a real copy here,
// as NullItem has no internal state.
func (i NullItem) Dup() StackItem {
	return i
}

// TryBytes implements StackItem interface.
func (i NullItem) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Null to ByteArray")
}

// TryInteger implements StackItem interface.
func (i NullItem) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Null to Integer")
}

// Equals implements StackItem interface.
func (i NullItem) Equals(s StackItem) bool {
	_, ok := s.(NullItem)
	return ok
}

// ToContractParameter implements StackItem interface.
func (i NullItem) ToContractParameter(map[StackItem]bool) smartcontract.Parameter {
	return smartcontract.Parameter{
		Type: smartcontract.AnyType,
	}
}

// Type implements StackItem interface.
func (i NullItem) Type() StackItemType { return AnyT }

// BigIntegerItem represents a big integer on the stack.
type BigIntegerItem struct {
	value *big.Int
}

// NewBigIntegerItem returns an new BigIntegerItem object.
func NewBigIntegerItem(value *big.Int) *BigIntegerItem {
	return &BigIntegerItem{
		value: value,
	}
}

// Bytes converts i to a slice of bytes.
func (i *BigIntegerItem) Bytes() []byte {
	return emit.IntToBytes(i.value)
}

// TryBytes implements StackItem interface.
func (i *BigIntegerItem) TryBytes() ([]byte, error) {
	return i.Bytes(), nil
}

// TryInteger implements StackItem interface.
func (i *BigIntegerItem) TryInteger() (*big.Int, error) {
	return i.value, nil
}

// Equals implements StackItem interface.
func (i *BigIntegerItem) Equals(s StackItem) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*BigIntegerItem)
	if ok {
		return i.value.Cmp(val.value) == 0
	}
	bs, err := s.TryBytes()
	return err == nil && bytes.Equal(i.Bytes(), bs)
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
func (i *BigIntegerItem) ToContractParameter(map[StackItem]bool) smartcontract.Parameter {
	return smartcontract.Parameter{
		Type:  smartcontract.IntegerType,
		Value: i.value.Int64(),
	}
}

// Type implements StackItem interface.
func (i *BigIntegerItem) Type() StackItemType { return IntegerT }

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
	return "Boolean"
}

// Dup implements StackItem interface.
func (i *BoolItem) Dup() StackItem {
	return &BoolItem{i.value}
}

// Bytes converts BoolItem to bytes.
func (i *BoolItem) Bytes() []byte {
	if i.value {
		return []byte{1}
	}
	return []byte{0}
}

// TryBytes implements StackItem interface.
func (i *BoolItem) TryBytes() ([]byte, error) {
	return i.Bytes(), nil
}

// TryInteger implements StackItem interface.
func (i *BoolItem) TryInteger() (*big.Int, error) {
	if i.value {
		return big.NewInt(1), nil
	}
	return big.NewInt(0), nil
}

// Equals implements StackItem interface.
func (i *BoolItem) Equals(s StackItem) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*BoolItem)
	if ok {
		return i.value == val.value
	}
	bs, err := s.TryBytes()
	return err == nil && bytes.Equal(i.Bytes(), bs)
}

// ToContractParameter implements StackItem interface.
func (i *BoolItem) ToContractParameter(map[StackItem]bool) smartcontract.Parameter {
	return smartcontract.Parameter{
		Type:  smartcontract.BoolType,
		Value: i.value,
	}
}

// Type implements StackItem interface.
func (i *BoolItem) Type() StackItemType { return BooleanT }

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

// TryBytes implements StackItem interface.
func (i *ByteArrayItem) TryBytes() ([]byte, error) {
	return i.value, nil
}

// TryInteger implements StackItem interface.
func (i *ByteArrayItem) TryInteger() (*big.Int, error) {
	return emit.BytesToInt(i.value), nil
}

// Equals implements StackItem interface.
func (i *ByteArrayItem) Equals(s StackItem) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	bs, err := s.TryBytes()
	return err == nil && bytes.Equal(i.value, bs)
}

// Dup implements StackItem interface.
func (i *ByteArrayItem) Dup() StackItem {
	a := make([]byte, len(i.value))
	copy(a, i.value)
	return &ByteArrayItem{a}
}

// ToContractParameter implements StackItem interface.
func (i *ByteArrayItem) ToContractParameter(map[StackItem]bool) smartcontract.Parameter {
	return smartcontract.Parameter{
		Type:  smartcontract.ByteArrayType,
		Value: i.value,
	}
}

// Type implements StackItem interface.
func (i *ByteArrayItem) Type() StackItemType { return ByteArrayT }

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

// TryBytes implements StackItem interface.
func (i *ArrayItem) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Array to ByteArray")
}

// TryInteger implements StackItem interface.
func (i *ArrayItem) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Array to Integer")
}

// Equals implements StackItem interface.
func (i *ArrayItem) Equals(s StackItem) bool {
	return i == s
}

// Dup implements StackItem interface.
func (i *ArrayItem) Dup() StackItem {
	// reference type
	return i
}

// ToContractParameter implements StackItem interface.
func (i *ArrayItem) ToContractParameter(seen map[StackItem]bool) smartcontract.Parameter {
	var value []smartcontract.Parameter

	if !seen[i] {
		seen[i] = true
		for _, stackItem := range i.value {
			parameter := stackItem.ToContractParameter(seen)
			value = append(value, parameter)
		}
	}
	return smartcontract.Parameter{
		Type:  smartcontract.ArrayType,
		Value: value,
	}
}

// Type implements StackItem interface.
func (i *ArrayItem) Type() StackItemType { return ArrayT }

// MapElement is a key-value pair of StackItems.
type MapElement struct {
	Key   StackItem
	Value StackItem
}

// MapItem represents Map object. It's ordered, so we use slice representation
// which should be fine for maps with less than 32 or so elements. Given that
// our VM has quite low limit of overall stack items, it should be good enough,
// but it can be extended with a real map for fast random access in the future
// if need be.
type MapItem struct {
	value []MapElement
}

// NewMapItem returns new MapItem object.
func NewMapItem() *MapItem {
	return &MapItem{
		value: make([]MapElement, 0),
	}
}

// Value implements StackItem interface.
func (i *MapItem) Value() interface{} {
	return i.value
}

// TryBytes implements StackItem interface.
func (i *MapItem) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Map to ByteArray")
}

// TryInteger implements StackItem interface.
func (i *MapItem) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Map to Integer")
}

// Equals implements StackItem interface.
func (i *MapItem) Equals(s StackItem) bool {
	return i == s
}

func (i *MapItem) String() string {
	return "Map"
}

// Index returns an index of the key in map.
func (i *MapItem) Index(key StackItem) int {
	for k := range i.value {
		if i.value[k].Key.Equals(key) {
			return k
		}
	}
	return -1
}

// Has checks if map has specified key.
func (i *MapItem) Has(key StackItem) bool {
	return i.Index(key) >= 0
}

// Dup implements StackItem interface.
func (i *MapItem) Dup() StackItem {
	// reference type
	return i
}

// ToContractParameter implements StackItem interface.
func (i *MapItem) ToContractParameter(seen map[StackItem]bool) smartcontract.Parameter {
	value := make([]smartcontract.ParameterPair, 0)
	if !seen[i] {
		seen[i] = true
		for k := range i.value {
			value = append(value, smartcontract.ParameterPair{
				Key:   i.value[k].Key.ToContractParameter(seen),
				Value: i.value[k].Value.ToContractParameter(seen),
			})
		}
	}
	return smartcontract.Parameter{
		Type:  smartcontract.MapType,
		Value: value,
	}
}

// Type implements StackItem interface.
func (i *MapItem) Type() StackItemType { return MapT }

// Add adds key-value pair to the map.
func (i *MapItem) Add(key, value StackItem) {
	if !isValidMapKey(key) {
		panic("wrong key type")
	}
	index := i.Index(key)
	if index >= 0 {
		i.value[index].Value = value
	} else {
		i.value = append(i.value, MapElement{key, value})
	}
}

// Drop removes given index from the map (no bounds check done here).
func (i *MapItem) Drop(index int) {
	copy(i.value[index:], i.value[index+1:])
	i.value = i.value[:len(i.value)-1]
}

// isValidMapKey checks whether it's possible to use given StackItem as a Map
// key.
func isValidMapKey(key StackItem) bool {
	switch key.(type) {
	case *BoolItem, *BigIntegerItem, *ByteArrayItem:
		return true
	default:
		return false
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

// TryBytes implements StackItem interface.
func (i *InteropItem) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Interop to ByteArray")
}

// TryInteger implements StackItem interface.
func (i *InteropItem) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Interop to Integer")
}

// Equals implements StackItem interface.
func (i *InteropItem) Equals(s StackItem) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*InteropItem)
	return ok && i.value == val.value
}

// ToContractParameter implements StackItem interface.
func (i *InteropItem) ToContractParameter(map[StackItem]bool) smartcontract.Parameter {
	return smartcontract.Parameter{
		Type:  smartcontract.InteropInterfaceType,
		Value: nil,
	}
}

// Type implements StackItem interface.
func (i *InteropItem) Type() StackItemType { return InteropT }

// MarshalJSON implements the json.Marshaler interface.
func (i *InteropItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}
