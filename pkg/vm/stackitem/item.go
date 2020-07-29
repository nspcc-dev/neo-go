package stackitem

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MaxBigIntegerSizeBits is the maximum size of BigInt item in bits.
const MaxBigIntegerSizeBits = 32 * 8

// MaxArraySize is the maximum array size allowed in the VM.
const MaxArraySize = 1024

// MaxSize is the maximum item size allowed in the VM.
const MaxSize = 1024 * 1024

// Item represents the "real" value that is pushed on the stack.
type Item interface {
	fmt.Stringer
	Value() interface{}
	// Dup duplicates current Item.
	Dup() Item
	// Bool converts Item to a boolean value.
	Bool() bool
	// TryBytes converts Item to a byte slice.
	TryBytes() ([]byte, error)
	// TryInteger converts Item to an integer.
	TryInteger() (*big.Int, error)
	// Equals checks if 2 StackItems are equal.
	Equals(s Item) bool
	// Type returns stack item type.
	Type() Type
	// Convert converts Item to another type.
	Convert(Type) (Item, error)
}

var errInvalidConversion = errors.New("invalid conversion type")

// Make tries to make appropriate stack item from provided value.
// It will panic if it's not possible.
func Make(v interface{}) Item {
	switch val := v.(type) {
	case int:
		return &BigInteger{
			value: big.NewInt(int64(val)),
		}
	case int64:
		return &BigInteger{
			value: big.NewInt(val),
		}
	case uint8:
		return &BigInteger{
			value: big.NewInt(int64(val)),
		}
	case uint16:
		return &BigInteger{
			value: big.NewInt(int64(val)),
		}
	case uint32:
		return &BigInteger{
			value: big.NewInt(int64(val)),
		}
	case uint64:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, val)
		bigInt := big.NewInt(0)
		bigInt.SetBytes(b)
		return &BigInteger{
			value: bigInt,
		}
	case []byte:
		return &ByteArray{
			value: val,
		}
	case string:
		return &ByteArray{
			value: []byte(val),
		}
	case bool:
		return &Bool{
			value: val,
		}
	case []Item:
		return &Array{
			value: val,
		}
	case *big.Int:
		return NewBigInteger(val)
	case Item:
		return val
	case []int:
		var a []Item
		for _, i := range val {
			a = append(a, Make(i))
		}
		return Make(a)
	default:
		i64T := reflect.TypeOf(int64(0))
		if reflect.TypeOf(val).ConvertibleTo(i64T) {
			i64Val := reflect.ValueOf(val).Convert(i64T).Interface()
			return Make(i64Val)
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

// convertPrimitive converts primitive item to a specified type.
func convertPrimitive(item Item, typ Type) (Item, error) {
	if item.Type() == typ {
		return item, nil
	}
	switch typ {
	case IntegerT:
		bi, err := item.TryInteger()
		if err != nil {
			return nil, err
		}
		return NewBigInteger(bi), nil
	case ByteArrayT, BufferT:
		b, err := item.TryBytes()
		if err != nil {
			return nil, err
		}
		if typ == BufferT {
			return NewBuffer(b), nil
		}
		return NewByteArray(b), nil
	case BooleanT:
		return NewBool(item.Bool()), nil
	default:
		return nil, errInvalidConversion
	}
}

// Struct represents a struct on the stack.
type Struct struct {
	value []Item
}

// NewStruct returns an new Struct object.
func NewStruct(items []Item) *Struct {
	return &Struct{
		value: items,
	}
}

// Value implements Item interface.
func (i *Struct) Value() interface{} {
	return i.value
}

// Remove removes element at `pos` index from Struct value.
// It will panics on bad index.
func (i *Struct) Remove(pos int) {
	i.value = append(i.value[:pos], i.value[pos+1:]...)
}

// Append adds Item at the end of Struct value.
func (i *Struct) Append(item Item) {
	i.value = append(i.value, item)
}

// Clear removes all elements from Struct item value.
func (i *Struct) Clear() {
	i.value = i.value[:0]
}

// Len returns length of Struct value.
func (i *Struct) Len() int {
	return len(i.value)
}

// String implements Item interface.
func (i *Struct) String() string {
	return "Struct"
}

// Dup implements Item interface.
func (i *Struct) Dup() Item {
	// it's a reference type, so no copying here.
	return i
}

// Bool implements Item interface.
func (i *Struct) Bool() bool { return true }

// TryBytes implements Item interface.
func (i *Struct) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Struct to ByteString")
}

// TryInteger implements Item interface.
func (i *Struct) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Struct to Integer")
}

// Equals implements Item interface.
func (i *Struct) Equals(s Item) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*Struct)
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

// Type implements Item interface.
func (i *Struct) Type() Type { return StructT }

// Convert implements Item interface.
func (i *Struct) Convert(typ Type) (Item, error) {
	switch typ {
	case StructT:
		return i, nil
	case ArrayT:
		arr := make([]Item, len(i.value))
		copy(arr, i.value)
		return NewArray(arr), nil
	case BooleanT:
		return NewBool(i.Bool()), nil
	default:
		return nil, errInvalidConversion
	}
}

// Clone returns a Struct with all Struct fields copied by value.
// Array fields are still copied by reference.
func (i *Struct) Clone() *Struct {
	ret := &Struct{make([]Item, len(i.value))}
	for j := range i.value {
		switch t := i.value[j].(type) {
		case *Struct:
			ret.value[j] = t.Clone()
		default:
			ret.value[j] = t
		}
	}
	return ret
}

// Null represents null on the stack.
type Null struct{}

// String implements Item interface.
func (i Null) String() string {
	return "Null"
}

// Value implements Item interface.
func (i Null) Value() interface{} {
	return nil
}

// Dup implements Item interface.
// There is no need to perform a real copy here,
// as Null has no internal state.
func (i Null) Dup() Item {
	return i
}

// Bool implements Item interface.
func (i Null) Bool() bool { return false }

// TryBytes implements Item interface.
func (i Null) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Null to ByteString")
}

// TryInteger implements Item interface.
func (i Null) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Null to Integer")
}

// Equals implements Item interface.
func (i Null) Equals(s Item) bool {
	_, ok := s.(Null)
	return ok
}

// Type implements Item interface.
func (i Null) Type() Type { return AnyT }

// Convert implements Item interface.
func (i Null) Convert(typ Type) (Item, error) {
	if typ == AnyT || !typ.IsValid() {
		return nil, errInvalidConversion
	}
	return i, nil
}

// BigInteger represents a big integer on the stack.
type BigInteger struct {
	value *big.Int
}

// NewBigInteger returns an new BigInteger object.
func NewBigInteger(value *big.Int) *BigInteger {
	if value.BitLen() > MaxBigIntegerSizeBits {
		panic("integer is too big")
	}
	return &BigInteger{
		value: value,
	}
}

// Bytes converts i to a slice of bytes.
func (i *BigInteger) Bytes() []byte {
	return bigint.ToBytes(i.value)
}

// Bool implements Item interface.
func (i *BigInteger) Bool() bool {
	return i.value.Sign() != 0
}

// TryBytes implements Item interface.
func (i *BigInteger) TryBytes() ([]byte, error) {
	return i.Bytes(), nil
}

// TryInteger implements Item interface.
func (i *BigInteger) TryInteger() (*big.Int, error) {
	return i.value, nil
}

// Equals implements Item interface.
func (i *BigInteger) Equals(s Item) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*BigInteger)
	return ok && i.value.Cmp(val.value) == 0
}

// Value implements Item interface.
func (i *BigInteger) Value() interface{} {
	return i.value
}

func (i *BigInteger) String() string {
	return "BigInteger"
}

// Dup implements Item interface.
func (i *BigInteger) Dup() Item {
	n := new(big.Int)
	return &BigInteger{n.Set(i.value)}
}

// Type implements Item interface.
func (i *BigInteger) Type() Type { return IntegerT }

// Convert implements Item interface.
func (i *BigInteger) Convert(typ Type) (Item, error) {
	return convertPrimitive(i, typ)
}

// MarshalJSON implements the json.Marshaler interface.
func (i *BigInteger) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

// Bool represents a boolean Item.
type Bool struct {
	value bool
}

// NewBool returns an new Bool object.
func NewBool(val bool) *Bool {
	return &Bool{
		value: val,
	}
}

// Value implements Item interface.
func (i *Bool) Value() interface{} {
	return i.value
}

// MarshalJSON implements the json.Marshaler interface.
func (i *Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

func (i *Bool) String() string {
	return "Boolean"
}

// Dup implements Item interface.
func (i *Bool) Dup() Item {
	return &Bool{i.value}
}

// Bool implements Item interface.
func (i *Bool) Bool() bool { return i.value }

// Bytes converts Bool to bytes.
func (i *Bool) Bytes() []byte {
	if i.value {
		return []byte{1}
	}
	return []byte{0}
}

// TryBytes implements Item interface.
func (i *Bool) TryBytes() ([]byte, error) {
	return i.Bytes(), nil
}

// TryInteger implements Item interface.
func (i *Bool) TryInteger() (*big.Int, error) {
	if i.value {
		return big.NewInt(1), nil
	}
	return big.NewInt(0), nil
}

// Equals implements Item interface.
func (i *Bool) Equals(s Item) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*Bool)
	return ok && i.value == val.value
}

// Type implements Item interface.
func (i *Bool) Type() Type { return BooleanT }

// Convert implements Item interface.
func (i *Bool) Convert(typ Type) (Item, error) {
	return convertPrimitive(i, typ)
}

// ByteArray represents a byte array on the stack.
type ByteArray struct {
	value []byte
}

// NewByteArray returns an new ByteArray object.
func NewByteArray(b []byte) *ByteArray {
	return &ByteArray{
		value: b,
	}
}

// Value implements Item interface.
func (i *ByteArray) Value() interface{} {
	return i.value
}

// MarshalJSON implements the json.Marshaler interface.
func (i *ByteArray) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(i.value))
}

func (i *ByteArray) String() string {
	return "ByteString"
}

// Bool implements Item interface.
func (i *ByteArray) Bool() bool {
	if len(i.value) > MaxBigIntegerSizeBits/8 {
		return true
	}
	for _, b := range i.value {
		if b != 0 {
			return true
		}
	}
	return false
}

// TryBytes implements Item interface.
func (i *ByteArray) TryBytes() ([]byte, error) {
	val := make([]byte, len(i.value))
	copy(val, i.value)
	return val, nil
}

// TryInteger implements Item interface.
func (i *ByteArray) TryInteger() (*big.Int, error) {
	if len(i.value) > MaxBigIntegerSizeBits/8 {
		return nil, errors.New("integer is too big")
	}
	return bigint.FromBytes(i.value), nil
}

// Equals implements Item interface.
func (i *ByteArray) Equals(s Item) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*ByteArray)
	return ok && bytes.Equal(i.value, val.value)
}

// Dup implements Item interface.
func (i *ByteArray) Dup() Item {
	a := make([]byte, len(i.value))
	copy(a, i.value)
	return &ByteArray{a}
}

// Type implements Item interface.
func (i *ByteArray) Type() Type { return ByteArrayT }

// Convert implements Item interface.
func (i *ByteArray) Convert(typ Type) (Item, error) {
	return convertPrimitive(i, typ)
}

// Array represents a new Array object.
type Array struct {
	value []Item
}

// NewArray returns a new Array object.
func NewArray(items []Item) *Array {
	return &Array{
		value: items,
	}
}

// Value implements Item interface.
func (i *Array) Value() interface{} {
	return i.value
}

// Remove removes element at `pos` index from Array value.
// It will panics on bad index.
func (i *Array) Remove(pos int) {
	i.value = append(i.value[:pos], i.value[pos+1:]...)
}

// Append adds Item at the end of Array value.
func (i *Array) Append(item Item) {
	i.value = append(i.value, item)
}

// Clear removes all elements from Array item value.
func (i *Array) Clear() {
	i.value = i.value[:0]
}

// Len returns length of Array value.
func (i *Array) Len() int {
	return len(i.value)
}

// MarshalJSON implements the json.Marshaler interface.
func (i *Array) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

func (i *Array) String() string {
	return "Array"
}

// Bool implements Item interface.
func (i *Array) Bool() bool { return true }

// TryBytes implements Item interface.
func (i *Array) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Array to ByteString")
}

// TryInteger implements Item interface.
func (i *Array) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Array to Integer")
}

// Equals implements Item interface.
func (i *Array) Equals(s Item) bool {
	return i == s
}

// Dup implements Item interface.
func (i *Array) Dup() Item {
	// reference type
	return i
}

// Type implements Item interface.
func (i *Array) Type() Type { return ArrayT }

// Convert implements Item interface.
func (i *Array) Convert(typ Type) (Item, error) {
	switch typ {
	case ArrayT:
		return i, nil
	case StructT:
		arr := make([]Item, len(i.value))
		copy(arr, i.value)
		return NewStruct(arr), nil
	case BooleanT:
		return NewBool(i.Bool()), nil
	default:
		return nil, errInvalidConversion
	}
}

// MapElement is a key-value pair of StackItems.
type MapElement struct {
	Key   Item
	Value Item
}

// Map represents Map object. It's ordered, so we use slice representation
// which should be fine for maps with less than 32 or so elements. Given that
// our VM has quite low limit of overall stack items, it should be good enough,
// but it can be extended with a real map for fast random access in the future
// if need be.
type Map struct {
	value []MapElement
}

// NewMap returns new Map object.
func NewMap() *Map {
	return &Map{
		value: make([]MapElement, 0),
	}
}

// NewMapWithValue returns new Map object filled with specified value.
func NewMapWithValue(value []MapElement) *Map {
	if value != nil {
		return &Map{
			value: value,
		}
	}
	return NewMap()
}

// Value implements Item interface.
func (i *Map) Value() interface{} {
	return i.value
}

// Clear removes all elements from Map item value.
func (i *Map) Clear() {
	i.value = i.value[:0]
}

// Len returns length of Map value.
func (i *Map) Len() int {
	return len(i.value)
}

// Bool implements Item interface.
func (i *Map) Bool() bool { return true }

// TryBytes implements Item interface.
func (i *Map) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Map to ByteString")
}

// TryInteger implements Item interface.
func (i *Map) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Map to Integer")
}

// Equals implements Item interface.
func (i *Map) Equals(s Item) bool {
	return i == s
}

func (i *Map) String() string {
	return "Map"
}

// Index returns an index of the key in map.
func (i *Map) Index(key Item) int {
	for k := range i.value {
		if i.value[k].Key.Equals(key) {
			return k
		}
	}
	return -1
}

// Has checks if map has specified key.
func (i *Map) Has(key Item) bool {
	return i.Index(key) >= 0
}

// Dup implements Item interface.
func (i *Map) Dup() Item {
	// reference type
	return i
}

// Type implements Item interface.
func (i *Map) Type() Type { return MapT }

// Convert implements Item interface.
func (i *Map) Convert(typ Type) (Item, error) {
	switch typ {
	case MapT:
		return i, nil
	case BooleanT:
		return NewBool(i.Bool()), nil
	default:
		return nil, errInvalidConversion
	}
}

// Add adds key-value pair to the map.
func (i *Map) Add(key, value Item) {
	if !IsValidMapKey(key) {
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
func (i *Map) Drop(index int) {
	copy(i.value[index:], i.value[index+1:])
	i.value = i.value[:len(i.value)-1]
}

// IsValidMapKey checks whether it's possible to use given Item as a Map
// key.
func IsValidMapKey(key Item) bool {
	switch key.(type) {
	case *Bool, *BigInteger, *ByteArray:
		return true
	default:
		return false
	}
}

// Interop represents interop data on the stack.
type Interop struct {
	value interface{}
}

// NewInterop returns new Interop object.
func NewInterop(value interface{}) *Interop {
	return &Interop{
		value: value,
	}
}

// Value implements Item interface.
func (i *Interop) Value() interface{} {
	return i.value
}

// String implements stringer interface.
func (i *Interop) String() string {
	return "Interop"
}

// Dup implements Item interface.
func (i *Interop) Dup() Item {
	// reference type
	return i
}

// Bool implements Item interface.
func (i *Interop) Bool() bool { return true }

// TryBytes implements Item interface.
func (i *Interop) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Interop to ByteString")
}

// TryInteger implements Item interface.
func (i *Interop) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Interop to Integer")
}

// Equals implements Item interface.
func (i *Interop) Equals(s Item) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*Interop)
	return ok && i.value == val.value
}

// Type implements Item interface.
func (i *Interop) Type() Type { return InteropT }

// Convert implements Item interface.
func (i *Interop) Convert(typ Type) (Item, error) {
	switch typ {
	case InteropT:
		return i, nil
	case BooleanT:
		return NewBool(i.Bool()), nil
	default:
		return nil, errInvalidConversion
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (i *Interop) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

// Pointer represents VM-level instruction pointer.
type Pointer struct {
	pos    int
	script []byte
	hash   util.Uint160
}

// NewPointer returns new pointer on the specified position.
func NewPointer(pos int, script []byte) *Pointer {
	return &Pointer{
		pos:    pos,
		script: script,
		hash:   hash.Hash160(script),
	}
}

// String implements Item interface.
func (p *Pointer) String() string {
	return "Pointer"
}

// Value implements Item interface.
func (p *Pointer) Value() interface{} {
	return p.pos
}

// Dup implements Item interface.
func (p *Pointer) Dup() Item {
	return &Pointer{
		pos:    p.pos,
		script: p.script,
		hash:   p.hash,
	}
}

// Bool implements Item interface.
func (p *Pointer) Bool() bool {
	return true
}

// TryBytes implements Item interface.
func (p *Pointer) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert Pointer to ByteString")
}

// TryInteger implements Item interface.
func (p *Pointer) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Pointer to Integer")
}

// Equals implements Item interface.
func (p *Pointer) Equals(s Item) bool {
	if p == s {
		return true
	}
	ptr, ok := s.(*Pointer)
	return ok && p.pos == ptr.pos && p.hash == ptr.hash
}

// Type implements Item interface.
func (p *Pointer) Type() Type {
	return PointerT
}

// Convert implements Item interface.
func (p *Pointer) Convert(typ Type) (Item, error) {
	switch typ {
	case PointerT:
		return p, nil
	case BooleanT:
		return NewBool(p.Bool()), nil
	default:
		return nil, errInvalidConversion
	}
}

// ScriptHash returns pointer item hash
func (p *Pointer) ScriptHash() util.Uint160 {
	return p.hash
}

// Position returns pointer item position
func (p *Pointer) Position() int {
	return p.pos
}

// Buffer represents represents Buffer stack item.
type Buffer struct {
	value []byte
}

// NewBuffer returns a new Buffer object.
func NewBuffer(b []byte) *Buffer {
	return &Buffer{
		value: b,
	}
}

// Value implements Item interface.
func (i *Buffer) Value() interface{} {
	return i.value
}

// String implements fmt.Stringer interface.
func (i *Buffer) String() string {
	return "Buffer"
}

// Bool implements Item interface.
func (i *Buffer) Bool() bool {
	return true
}

// TryBytes implements Item interface.
func (i *Buffer) TryBytes() ([]byte, error) {
	val := make([]byte, len(i.value))
	copy(val, i.value)
	return val, nil
}

// TryInteger implements Item interface.
func (i *Buffer) TryInteger() (*big.Int, error) {
	return nil, errors.New("can't convert Buffer to Integer")
}

// Equals implements Item interface.
func (i *Buffer) Equals(s Item) bool {
	return i == s
}

// Dup implements Item interface.
func (i *Buffer) Dup() Item {
	return i
}

// MarshalJSON implements the json.Marshaler interface.
func (i *Buffer) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(i.value))
}

// Type implements Item interface.
func (i *Buffer) Type() Type { return BufferT }

// Convert implements Item interface.
func (i *Buffer) Convert(typ Type) (Item, error) {
	switch typ {
	case BooleanT:
		return NewBool(i.Bool()), nil
	case BufferT:
		return i, nil
	case ByteArrayT:
		val := make([]byte, len(i.value))
		copy(val, i.value)
		return NewByteArray(val), nil
	case IntegerT:
		if len(i.value) > MaxBigIntegerSizeBits/8 {
			return nil, errInvalidConversion
		}
		return NewBigInteger(bigint.FromBytes(i.value)), nil
	default:
		return nil, errInvalidConversion
	}
}

// Len returns length of Buffer value.
func (i *Buffer) Len() int {
	return len(i.value)
}
