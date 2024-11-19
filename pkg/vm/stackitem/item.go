package stackitem

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"slices"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// MaxBigIntegerSizeBits is the maximum size of a BigInt item in bits.
	MaxBigIntegerSizeBits = 32 * 8
	// MaxSize is the maximum item size allowed in the VM.
	MaxSize = math.MaxUint16 * 2
	// MaxComparableNumOfItems is the maximum number of items that can be compared for structs.
	MaxComparableNumOfItems = MaxDeserialized
	// MaxClonableNumOfItems is the maximum number of items that can be cloned in structs.
	MaxClonableNumOfItems = MaxDeserialized
	// MaxByteArrayComparableSize is the maximum allowed length of a ByteArray for Equals method.
	// It is set to be the maximum uint16 value + 1.
	MaxByteArrayComparableSize = math.MaxUint16 + 1
	// MaxKeySize is the maximum size of a map key.
	MaxKeySize = 64
)

// Item represents the "real" value that is pushed on the stack.
type Item interface {
	fmt.Stringer
	Value() any
	// Dup duplicates current Item.
	Dup() Item
	// TryBool converts Item to a boolean value.
	TryBool() (bool, error)
	// TryBytes converts Item to a byte slice. If the underlying type is a
	// byte slice, it's returned as is without copying.
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

// Convertible is something that can be converted to/from Item.
type Convertible interface {
	ToStackItem() (Item, error)
	FromStackItem(Item) error
}

// Equatable describes a special value of Interop that can be compared with
// value of some other Interop that implements Equatable.
type Equatable interface {
	// Equals checks if two objects are equal.
	Equals(other Equatable) bool
}

var (
	// ErrInvalidConversion is returned upon an attempt to make an incorrect
	// conversion between item types.
	ErrInvalidConversion = errors.New("invalid conversion")

	// ErrTooBig is returned when an item exceeds some size constraints, like
	// the maximum allowed integer value of the number of elements in an array. It
	// can also be returned by serialization functions if the resulting
	// value exceeds MaxSize.
	ErrTooBig = errors.New("too big")
	// ErrReadOnly is returned on attempt to modify immutable stack item.
	ErrReadOnly = errors.New("item is read-only")

	errTooBigComparable = fmt.Errorf("%w: uncomparable", ErrTooBig)
	errTooBigInteger    = fmt.Errorf("%w: integer", ErrTooBig)
	errTooBigKey        = fmt.Errorf("%w: map key", ErrTooBig)
	errTooBigSize       = fmt.Errorf("%w: size", ErrTooBig)
	errTooBigElements   = fmt.Errorf("%w: many elements", ErrTooBig)
)

// mkInvConversion creates a conversion error with additional metadata (from and
// to types).
func mkInvConversion(from Item, to Type) error {
	return fmt.Errorf("%w: %s/%s", ErrInvalidConversion, from, to)
}

// Make tries to make an appropriate stack item from the provided value.
// It will panic if it's not possible.
func Make(v any) Item {
	switch val := v.(type) {
	case int:
		return (*BigInteger)(big.NewInt(int64(val)))
	case int64:
		return (*BigInteger)(big.NewInt(val))
	case uint8:
		return (*BigInteger)(big.NewInt(int64(val)))
	case uint16:
		return (*BigInteger)(big.NewInt(int64(val)))
	case uint32:
		return (*BigInteger)(big.NewInt(int64(val)))
	case uint64:
		return (*BigInteger)(new(big.Int).SetUint64(val))
	case []byte:
		return NewByteArray(val)
	case string:
		return NewByteArray([]byte(val))
	case bool:
		return Bool(val)
	case []Item:
		return &Array{
			value: val,
		}
	case *big.Int:
		return NewBigInteger(val)
	case Item:
		return val
	case []int:
		var res = make([]Item, len(val))
		for i := range val {
			res[i] = Make(val[i])
		}
		return Make(res)
	case []string:
		var res = make([]Item, len(val))
		for i := range val {
			res[i] = Make(val[i])
		}
		return Make(res)
	case []any:
		res := make([]Item, len(val))
		for i := range val {
			res[i] = Make(val[i])
		}
		return Make(res)
	case util.Uint160:
		return Make(val.BytesBE())
	case util.Uint256:
		return Make(val.BytesBE())
	case *util.Uint160:
		if val == nil {
			return Null{}
		}
		return Make(*val)
	case *util.Uint256:
		if val == nil {
			return Null{}
		}
		return Make(*val)
	case nil:
		return Null{}
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

// ToString converts an Item to a string if it is a valid UTF-8.
func ToString(item Item) (string, error) {
	bs, err := item.TryBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(bs) {
		return "", fmt.Errorf("%w: not UTF-8", ErrInvalidValue)
	}
	return string(bs), nil
}

// convertPrimitive converts a primitive item to the specified type.
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
			return NewBuffer(bytes.Clone(b)), nil
		}
		// ByteArray can't really be changed, so it's OK to reuse `b`.
		return NewByteArray(b), nil
	case BooleanT:
		b, err := item.TryBool()
		if err != nil {
			return nil, err
		}
		return NewBool(b), nil
	default:
		return nil, mkInvConversion(item, typ)
	}
}

// Struct represents a struct on the stack.
type Struct struct {
	value []Item
	rc
	ro
}

// NewStruct returns a new Struct object.
func NewStruct(items []Item) *Struct {
	return &Struct{
		value: items,
	}
}

// Value implements the Item interface.
func (i *Struct) Value() any {
	return i.value
}

// Remove removes the element at `pos` index from the Struct value.
// It will panic if a bad index given.
func (i *Struct) Remove(pos int) {
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
	i.value = append(i.value[:pos], i.value[pos+1:]...)
}

// Append adds an Item to the end of the Struct value.
func (i *Struct) Append(item Item) {
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
	i.value = append(i.value, item)
}

// Clear removes all elements from the Struct item value.
func (i *Struct) Clear() {
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
	i.value = i.value[:0]
}

// Len returns the length of the Struct value.
func (i *Struct) Len() int {
	return len(i.value)
}

// String implements the Item interface.
func (i *Struct) String() string {
	return "Struct"
}

// Dup implements the Item interface.
func (i *Struct) Dup() Item {
	// it's a reference type, so no copying here.
	return i
}

// TryBool implements the Item interface.
func (i *Struct) TryBool() (bool, error) { return true, nil }

// TryBytes implements the Item interface.
func (i *Struct) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements the Item interface.
func (i *Struct) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
}

// Equals implements the Item interface.
func (i *Struct) Equals(s Item) bool {
	if s == nil {
		return false
	}
	val, ok := s.(*Struct)
	if !ok {
		return false
	}
	var limit = MaxComparableNumOfItems - 1 // 1 for current element.
	return i.equalStruct(val, &limit)
}

func (i *Struct) equalStruct(s *Struct, limit *int) bool {
	if i == s {
		return true
	} else if len(i.value) != len(s.value) {
		return false
	}
	var maxComparableSize = MaxByteArrayComparableSize
	for j := range i.value {
		*limit--
		if *limit == 0 {
			panic(errTooBigElements)
		}
		arr, ok := i.value[j].(*ByteArray)
		if ok {
			if !arr.equalsLimited(s.value[j], &maxComparableSize) {
				return false
			}
		} else {
			if maxComparableSize == 0 {
				panic(errTooBigComparable)
			}
			maxComparableSize--
			sa, oka := i.value[j].(*Struct)
			sb, okb := s.value[j].(*Struct)
			if oka && okb {
				if !sa.equalStruct(sb, limit) {
					return false
				}
			} else if !i.value[j].Equals(s.value[j]) {
				return false
			}
		}
	}
	return true
}

// Type implements the Item interface.
func (i *Struct) Type() Type { return StructT }

// Convert implements the Item interface.
func (i *Struct) Convert(typ Type) (Item, error) {
	switch typ {
	case StructT:
		return i, nil
	case ArrayT:
		return NewArray(slices.Clone(i.value)), nil
	case BooleanT:
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(i, typ)
	}
}

// Clone returns a Struct with all Struct fields copied by the value.
// Array fields are still copied by reference.
func (i *Struct) Clone() (*Struct, error) {
	var limit = MaxClonableNumOfItems - 1 // For this struct itself.
	return i.clone(&limit)
}

func (i *Struct) clone(limit *int) (*Struct, error) {
	ret := &Struct{value: make([]Item, len(i.value))}
	for j := range i.value {
		*limit--
		if *limit < 0 {
			return nil, ErrTooBig
		}
		switch t := i.value[j].(type) {
		case *Struct:
			var err error

			ret.value[j], err = t.clone(limit)
			if err != nil {
				return nil, err
			}
		default:
			ret.value[j] = t
		}
	}
	return ret, nil
}

// Null represents null on the stack.
type Null struct{}

// String implements the Item interface.
func (i Null) String() string {
	return "Null"
}

// Value implements the Item interface.
func (i Null) Value() any {
	return nil
}

// Dup implements the Item interface.
// There is no need to perform a real copy here
// since Null has no internal state.
func (i Null) Dup() Item {
	return i
}

// TryBool implements the Item interface.
func (i Null) TryBool() (bool, error) { return false, nil }

// TryBytes implements the Item interface.
func (i Null) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements the Item interface.
func (i Null) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
}

// Equals implements the Item interface.
func (i Null) Equals(s Item) bool {
	_, ok := s.(Null)
	return ok
}

// Type implements the Item interface.
func (i Null) Type() Type { return AnyT }

// Convert implements the Item interface.
func (i Null) Convert(typ Type) (Item, error) {
	if typ == AnyT || !typ.IsValid() {
		return nil, mkInvConversion(i, typ)
	}
	return i, nil
}

// BigInteger represents a big integer on the stack.
type BigInteger big.Int

// NewBigInteger returns an new BigInteger object.
func NewBigInteger(value *big.Int) *BigInteger {
	if err := CheckIntegerSize(value); err != nil {
		panic(err)
	}
	return (*BigInteger)(value)
}

// CheckIntegerSize checks that the value size doesn't exceed the VM limit for Interer.
func CheckIntegerSize(value *big.Int) error {
	// There are 2 cases when `BitLen` differs from the actual size:
	// 1. Positive integer with the highest bit on byte boundary = 1.
	// 2. Negative integer with the highest bit on byte boundary = 1
	//    minus some value. (-0x80 -> 0x80, -0x7F -> 0x81, -0x81 -> 0x7FFF).
	sz := value.BitLen()
	// This check is not required, just an optimization for the common case.
	if sz < MaxBigIntegerSizeBits {
		return nil
	}
	if sz > MaxBigIntegerSizeBits {
		return errTooBigInteger
	}
	if value.Sign() == 1 || value.TrailingZeroBits() != MaxBigIntegerSizeBits-1 {
		return errTooBigInteger
	}
	return nil
}

// Big casts i to the big.Int type.
func (i *BigInteger) Big() *big.Int {
	return (*big.Int)(i)
}

// Bytes converts i to a slice of bytes.
func (i *BigInteger) Bytes() []byte {
	return bigint.ToBytes(i.Big())
}

// TryBool implements the Item interface.
func (i *BigInteger) TryBool() (bool, error) {
	return i.Big().Sign() != 0, nil
}

// TryBytes implements the Item interface.
func (i *BigInteger) TryBytes() ([]byte, error) {
	return i.Bytes(), nil
}

// TryInteger implements the Item interface.
func (i *BigInteger) TryInteger() (*big.Int, error) {
	return i.Big(), nil
}

// Equals implements the Item interface.
func (i *BigInteger) Equals(s Item) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*BigInteger)
	return ok && i.Big().Cmp(val.Big()) == 0
}

// Value implements the Item interface.
func (i *BigInteger) Value() any {
	return i.Big()
}

func (i *BigInteger) String() string {
	return "BigInteger"
}

// Dup implements the Item interface.
func (i *BigInteger) Dup() Item {
	n := new(big.Int)
	return (*BigInteger)(n.Set(i.Big()))
}

// Type implements the Item interface.
func (i *BigInteger) Type() Type { return IntegerT }

// Convert implements the Item interface.
func (i *BigInteger) Convert(typ Type) (Item, error) {
	return convertPrimitive(i, typ)
}

// MarshalJSON implements the json.Marshaler interface.
func (i *BigInteger) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.Big())
}

// Bool represents a boolean Item.
type Bool bool

// NewBool returns an new Bool object.
func NewBool(val bool) Bool {
	return Bool(val)
}

// Value implements the Item interface.
func (i Bool) Value() any {
	return bool(i)
}

// MarshalJSON implements the json.Marshaler interface.
func (i Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(bool(i))
}

func (i Bool) String() string {
	return "Boolean"
}

// Dup implements the Item interface.
func (i Bool) Dup() Item {
	return i
}

// TryBool implements the Item interface.
func (i Bool) TryBool() (bool, error) { return bool(i), nil }

// Bytes converts Bool to bytes.
func (i Bool) Bytes() []byte {
	if i {
		return []byte{1}
	}
	return []byte{0}
}

// TryBytes implements the Item interface.
func (i Bool) TryBytes() ([]byte, error) {
	return i.Bytes(), nil
}

// TryInteger implements the Item interface.
func (i Bool) TryInteger() (*big.Int, error) {
	if i {
		return big.NewInt(1), nil
	}
	return big.NewInt(0), nil
}

// Equals implements the Item interface.
func (i Bool) Equals(s Item) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(Bool)
	return ok && i == val
}

// Type implements the Item interface.
func (i Bool) Type() Type { return BooleanT }

// Convert implements the Item interface.
func (i Bool) Convert(typ Type) (Item, error) {
	return convertPrimitive(i, typ)
}

// ByteArray represents a byte array on the stack.
type ByteArray []byte

// NewByteArray returns an new ByteArray object.
func NewByteArray(b []byte) *ByteArray {
	return (*ByteArray)(&b)
}

// Value implements the Item interface.
func (i *ByteArray) Value() any {
	return []byte(*i)
}

// MarshalJSON implements the json.Marshaler interface.
func (i *ByteArray) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(*i))
}

func (i *ByteArray) String() string {
	return "ByteString"
}

// TryBool implements the Item interface.
func (i *ByteArray) TryBool() (bool, error) {
	if len(*i) > MaxBigIntegerSizeBits/8 {
		return false, errTooBigInteger
	}
	for _, b := range *i {
		if b != 0 {
			return true, nil
		}
	}
	return false, nil
}

// TryBytes implements the Item interface.
func (i ByteArray) TryBytes() ([]byte, error) {
	return i, nil
}

// TryInteger implements the Item interface.
func (i ByteArray) TryInteger() (*big.Int, error) {
	if len(i) > MaxBigIntegerSizeBits/8 {
		return nil, errTooBigInteger
	}
	return bigint.FromBytes(i), nil
}

// Equals implements the Item interface.
func (i *ByteArray) Equals(s Item) bool {
	var limit = MaxByteArrayComparableSize
	return i.equalsLimited(s, &limit)
}

// equalsLimited compares ByteArray with provided stackitem using the limit.
func (i *ByteArray) equalsLimited(s Item, limit *int) bool {
	if i == nil {
		return s == nil
	}
	lCurr := len(*i)
	if lCurr > *limit || *limit == 0 {
		panic(errTooBigComparable)
	}

	var comparedSize = 1
	defer func() { *limit -= comparedSize }()

	if s == nil {
		return false
	}
	val, ok := s.(*ByteArray)
	if !ok {
		return false
	}
	lOther := len(*val)
	comparedSize = max(lCurr, lOther)

	if i == val {
		return true
	}
	if lOther > *limit {
		panic(errTooBigComparable)
	}
	return bytes.Equal(*i, *val)
}

// Dup implements the Item interface.
func (i *ByteArray) Dup() Item {
	ba := bytes.Clone(*i)
	return (*ByteArray)(&ba)
}

// Type implements the Item interface.
func (i *ByteArray) Type() Type { return ByteArrayT }

// Convert implements the Item interface.
func (i *ByteArray) Convert(typ Type) (Item, error) {
	return convertPrimitive(i, typ)
}

// Array represents a new Array object.
type Array struct {
	value []Item
	rc
	ro
}

// NewArray returns a new Array object.
func NewArray(items []Item) *Array {
	return &Array{
		value: items,
	}
}

// Value implements the Item interface.
func (i *Array) Value() any {
	return i.value
}

// Remove removes the element at `pos` index from Array value.
// It will panics on bad index.
func (i *Array) Remove(pos int) {
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
	i.value = append(i.value[:pos], i.value[pos+1:]...)
}

// Append adds an Item to the end of the Array value.
func (i *Array) Append(item Item) {
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
	i.value = append(i.value, item)
}

// Clear removes all elements from the Array item value.
func (i *Array) Clear() {
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
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

// TryBool implements the Item interface.
func (i *Array) TryBool() (bool, error) { return true, nil }

// TryBytes implements the Item interface.
func (i *Array) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements the Item interface.
func (i *Array) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
}

// Equals implements the Item interface.
func (i *Array) Equals(s Item) bool {
	return i == s
}

// Dup implements the Item interface.
func (i *Array) Dup() Item {
	// reference type
	return i
}

// Type implements the Item interface.
func (i *Array) Type() Type { return ArrayT }

// Convert implements the Item interface.
func (i *Array) Convert(typ Type) (Item, error) {
	switch typ {
	case ArrayT:
		return i, nil
	case StructT:
		return NewStruct(slices.Clone(i.value)), nil
	case BooleanT:
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(i, typ)
	}
}

// MapElement is a key-value pair of StackItems.
type MapElement struct {
	Key   Item
	Value Item
}

// Map represents a Map object. It's ordered, so we use slice representation,
// which should be fine for maps with less than 32 or so elements. Given that
// our VM has quite low limit of overall stack items, it should be good enough,
// but it can be extended with a real map for fast random access in the future
// if needed.
type Map struct {
	value []MapElement
	rc
	ro
}

// NewMap returns a new Map object.
func NewMap() *Map {
	return &Map{
		value: make([]MapElement, 0),
	}
}

// NewMapWithValue returns a new Map object filled with the specified value
// without value validation.
func NewMapWithValue(value []MapElement) *Map {
	if value != nil {
		return &Map{
			value: value,
		}
	}
	return NewMap()
}

// Value implements the Item interface.
func (i *Map) Value() any {
	return i.value
}

// Clear removes all elements from the Map item value.
func (i *Map) Clear() {
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
	i.value = i.value[:0]
}

// Len returns the length of the Map value.
func (i *Map) Len() int {
	return len(i.value)
}

// TryBool implements the Item interface.
func (i *Map) TryBool() (bool, error) { return true, nil }

// TryBytes implements the Item interface.
func (i *Map) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements the Item interface.
func (i *Map) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
}

// Equals implements the Item interface.
func (i *Map) Equals(s Item) bool {
	return i == s
}

func (i *Map) String() string {
	return "Map"
}

// Index returns an index of the key in map.
func (i *Map) Index(key Item) int {
	return slices.IndexFunc(i.value, func(e MapElement) bool {
		return e.Key.Equals(key)
	})
}

// Has checks if the map has the specified key.
func (i *Map) Has(key Item) bool {
	return i.Index(key) >= 0
}

// Dup implements the Item interface.
func (i *Map) Dup() Item {
	// reference type
	return i
}

// Type implements the Item interface.
func (i *Map) Type() Type { return MapT }

// Convert implements the Item interface.
func (i *Map) Convert(typ Type) (Item, error) {
	switch typ {
	case MapT:
		return i, nil
	case BooleanT:
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(i, typ)
	}
}

// Add adds a key-value pair to the map.
func (i *Map) Add(key, value Item) {
	if err := IsValidMapKey(key); err != nil {
		panic(err)
	}
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
	index := i.Index(key)
	if index >= 0 {
		i.value[index].Value = value
	} else {
		i.value = append(i.value, MapElement{key, value})
	}
}

// Drop removes the given index from the map (no bounds check done here).
func (i *Map) Drop(index int) {
	if i.IsReadOnly() {
		panic(ErrReadOnly)
	}
	copy(i.value[index:], i.value[index+1:])
	i.value = i.value[:len(i.value)-1]
}

// IsValidMapKey checks whether it's possible to use the given Item as a Map
// key.
func IsValidMapKey(key Item) error {
	switch key.(type) {
	case Bool, *BigInteger:
		return nil
	case *ByteArray:
		size := len(key.Value().([]byte))
		if size > MaxKeySize {
			return errTooBigKey
		}
		return nil
	default:
		return fmt.Errorf("%w: %s map key", ErrInvalidType, key.Type())
	}
}

// Interop represents interop data on the stack.
type Interop struct {
	value any
}

// NewInterop returns a new Interop object.
func NewInterop(value any) *Interop {
	return &Interop{
		value: value,
	}
}

// Value implements the Item interface.
func (i *Interop) Value() any {
	return i.value
}

// String implements stringer interface.
func (i *Interop) String() string {
	return "InteropInterface"
}

// Dup implements the Item interface.
func (i *Interop) Dup() Item {
	// reference type
	return i
}

// TryBool implements the Item interface.
func (i *Interop) TryBool() (bool, error) { return true, nil }

// TryBytes implements the Item interface.
func (i *Interop) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements the Item interface.
func (i *Interop) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
}

// Equals implements the Item interface.
func (i *Interop) Equals(s Item) bool {
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*Interop)
	if !ok {
		return false
	}
	a, okA := i.value.(Equatable)
	b, okB := val.value.(Equatable)
	return (okA && okB && a.Equals(b)) || (!okA && !okB && i.value == val.value)
}

// Type implements the Item interface.
func (i *Interop) Type() Type { return InteropT }

// Convert implements the Item interface.
func (i *Interop) Convert(typ Type) (Item, error) {
	switch typ {
	case InteropT:
		return i, nil
	case BooleanT:
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(i, typ)
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (i *Interop) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.value)
}

// Pointer represents a VM-level instruction pointer.
type Pointer struct {
	pos    int
	script []byte
	hash   util.Uint160
}

// NewPointer returns a new pointer on the specified position.
func NewPointer(pos int, script []byte) *Pointer {
	return &Pointer{
		pos:    pos,
		script: script,
		hash:   hash.Hash160(script),
	}
}

// NewPointerWithHash returns a new pointer on the specified position of the
// specified script. It differs from NewPointer in that the script hash is being
// passed explicitly to save on hash calculation. This hash is then being used
// for pointer comparisons.
func NewPointerWithHash(pos int, script []byte, h util.Uint160) *Pointer {
	return &Pointer{
		pos:    pos,
		script: script,
		hash:   h,
	}
}

// String implements the Item interface.
func (p *Pointer) String() string {
	return "Pointer"
}

// Value implements the Item interface.
func (p *Pointer) Value() any {
	return p.pos
}

// Dup implements the Item interface.
func (p *Pointer) Dup() Item {
	return &Pointer{
		pos:    p.pos,
		script: p.script,
		hash:   p.hash,
	}
}

// TryBool implements the Item interface.
func (p *Pointer) TryBool() (bool, error) {
	return true, nil
}

// TryBytes implements the Item interface.
func (p *Pointer) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(p, ByteArrayT)
}

// TryInteger implements the Item interface.
func (p *Pointer) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(p, IntegerT)
}

// Equals implements the Item interface.
func (p *Pointer) Equals(s Item) bool {
	if p == s {
		return true
	}
	ptr, ok := s.(*Pointer)
	return ok && p.pos == ptr.pos && p.hash == ptr.hash
}

// Type implements the Item interface.
func (p *Pointer) Type() Type {
	return PointerT
}

// Convert implements the Item interface.
func (p *Pointer) Convert(typ Type) (Item, error) {
	switch typ {
	case PointerT:
		return p, nil
	case BooleanT:
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(p, typ)
	}
}

// ScriptHash returns the pointer item hash.
func (p *Pointer) ScriptHash() util.Uint160 {
	return p.hash
}

// Position returns the pointer item position.
func (p *Pointer) Position() int {
	return p.pos
}

// Buffer represents a Buffer stack item.
type Buffer []byte

// NewBuffer returns a new Buffer object.
func NewBuffer(b []byte) *Buffer {
	return (*Buffer)(&b)
}

// Value implements the Item interface.
func (i *Buffer) Value() any {
	return []byte(*i)
}

// String implements the fmt.Stringer interface.
func (i *Buffer) String() string {
	return "Buffer"
}

// TryBool implements the Item interface.
func (i *Buffer) TryBool() (bool, error) {
	return true, nil
}

// TryBytes implements the Item interface.
func (i *Buffer) TryBytes() ([]byte, error) {
	return *i, nil
}

// TryInteger implements the Item interface.
func (i *Buffer) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
}

// Equals implements the Item interface.
func (i *Buffer) Equals(s Item) bool {
	return i == s
}

// Dup implements the Item interface.
func (i *Buffer) Dup() Item {
	return i
}

// MarshalJSON implements the json.Marshaler interface.
func (i *Buffer) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(*i))
}

// Type implements the Item interface.
func (i *Buffer) Type() Type { return BufferT }

// Convert implements the Item interface.
func (i *Buffer) Convert(typ Type) (Item, error) {
	switch typ {
	case BooleanT:
		return NewBool(true), nil
	case BufferT:
		return i, nil
	case ByteArrayT:
		return NewByteArray(bytes.Clone(*i)), nil
	case IntegerT:
		if len(*i) > MaxBigIntegerSizeBits/8 {
			return nil, errTooBigInteger
		}
		return NewBigInteger(bigint.FromBytes(*i)), nil
	default:
		return nil, mkInvConversion(i, typ)
	}
}

// Len returns the length of the Buffer value.
func (i *Buffer) Len() int {
	return len(*i)
}

// DeepCopy returns a new deep copy of the provided item.
// Values of Interop items are not deeply copied.
// It does preserve duplicates only for non-primitive types.
func DeepCopy(item Item, asImmutable bool) Item {
	seen := make(map[Item]Item, typicalNumOfItems)
	return deepCopy(item, seen, asImmutable)
}

func deepCopy(item Item, seen map[Item]Item, asImmutable bool) Item {
	if it := seen[item]; it != nil {
		return it
	}
	switch it := item.(type) {
	case Null:
		return Null{}
	case *Array:
		arr := NewArray(make([]Item, len(it.value)))
		seen[item] = arr
		for i := range it.value {
			arr.value[i] = deepCopy(it.value[i], seen, asImmutable)
		}
		arr.MarkAsReadOnly()
		return arr
	case *Struct:
		arr := NewStruct(make([]Item, len(it.value)))
		seen[item] = arr
		for i := range it.value {
			arr.value[i] = deepCopy(it.value[i], seen, asImmutable)
		}
		arr.MarkAsReadOnly()
		return arr
	case *Map:
		m := NewMap()
		seen[item] = m
		for i := range it.value {
			key := deepCopy(it.value[i].Key, seen,
				false) // Key is always primitive and not a Buffer.
			value := deepCopy(it.value[i].Value, seen, asImmutable)
			m.Add(key, value)
		}
		m.MarkAsReadOnly()
		return m
	case *BigInteger:
		bi := new(big.Int).Set(it.Big())
		return (*BigInteger)(bi)
	case *ByteArray:
		return NewByteArray(bytes.Clone(*it))
	case *Buffer:
		if asImmutable {
			return NewByteArray(bytes.Clone(*it))
		}
		return NewBuffer(bytes.Clone(*it))
	case Bool:
		return it
	case *Pointer:
		return NewPointerWithHash(it.pos, it.script, it.hash)
	case *Interop:
		return NewInterop(it.value)
	default:
		return nil
	}
}
