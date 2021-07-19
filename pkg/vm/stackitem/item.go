package stackitem

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// MaxBigIntegerSizeBits is the maximum size of BigInt item in bits.
	MaxBigIntegerSizeBits = 32 * 8
	// MaxArraySize is the maximum array size allowed in the VM.
	MaxArraySize = 1024
	// MaxSize is the maximum item size allowed in the VM.
	MaxSize = 1024 * 1024
	// MaxComparableNumOfItems is the maximum number of items that can be compared for structs.
	MaxComparableNumOfItems = 2048
	// MaxByteArrayComparableSize is the maximum allowed length of ByteArray for Equals method.
	// It is set to be the maximum uint16 value.
	MaxByteArrayComparableSize = math.MaxUint16
	// MaxKeySize is the maximum size of map key.
	MaxKeySize = 64
)

// Item represents the "real" value that is pushed on the stack.
type Item interface {
	fmt.Stringer
	Value() interface{}
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

var (
	// ErrInvalidConversion is returned on attempt to make an incorrect
	// conversion between item types.
	ErrInvalidConversion = errors.New("invalid conversion")

	// ErrTooBig is returned when item exceeds some size constraints like
	// maximum allowed integer value of number of elements in array. It
	// can also be returned by serialization functions if resulting
	// value exceeds MaxSize.
	ErrTooBig = errors.New("too big")

	errTooBigArray      = fmt.Errorf("%w: array", ErrTooBig)
	errTooBigComparable = fmt.Errorf("%w: uncomparable", ErrTooBig)
	errTooBigInteger    = fmt.Errorf("%w: integer", ErrTooBig)
	errTooBigKey        = fmt.Errorf("%w: map key", ErrTooBig)
	errTooBigSize       = fmt.Errorf("%w: size", ErrTooBig)
	errTooBigElements   = fmt.Errorf("%w: many elements", ErrTooBig)
)

// mkInvConversion creates conversion error with additional metadata (from and
// to types).
func mkInvConversion(from Item, to Type) error {
	return fmt.Errorf("%w: %s/%s", ErrInvalidConversion, from, to)
}

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

// ToString converts Item to string if it is a valid UTF-8.
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
			newb := make([]byte, len(b))
			copy(newb, b)
			return NewBuffer(newb), nil
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

// TryBool implements Item interface.
func (i *Struct) TryBool() (bool, error) { return true, nil }

// TryBytes implements Item interface.
func (i *Struct) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements Item interface.
func (i *Struct) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
}

// Equals implements Item interface.
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
	for j := range i.value {
		*limit--
		if *limit == 0 {
			panic(errTooBigElements)
		}
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
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(i, typ)
	}
}

// Clone returns a Struct with all Struct fields copied by value.
// Array fields are still copied by reference.
func (i *Struct) Clone(limit int) (*Struct, error) {
	return i.clone(&limit)
}

func (i *Struct) clone(limit *int) (*Struct, error) {
	ret := &Struct{make([]Item, len(i.value))}
	for j := range i.value {
		switch t := i.value[j].(type) {
		case *Struct:
			var err error

			ret.value[j], err = t.clone(limit)
			if err != nil {
				return nil, err
			}
			*limit--
		default:
			ret.value[j] = t
		}
		if *limit < 0 {
			return nil, ErrTooBig
		}
	}
	return ret, nil
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

// TryBool implements Item interface.
func (i Null) TryBool() (bool, error) { return false, nil }

// TryBytes implements Item interface.
func (i Null) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements Item interface.
func (i Null) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
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
		return nil, mkInvConversion(i, typ)
	}
	return i, nil
}

// BigInteger represents a big integer on the stack.
type BigInteger struct {
	value *big.Int
}

// NewBigInteger returns an new BigInteger object.
func NewBigInteger(value *big.Int) *BigInteger {
	// There are 2 cases, when `BitLen` differs from actual size:
	// 1. Positive integer with highest bit on byte boundary = 1.
	// 2. Negative integer with highest bit on byte boundary = 1
	//    minus some value. (-0x80 -> 0x80, -0x7F -> 0x81, -0x81 -> 0x7FFF).
	sz := value.BitLen()
	if sz > MaxBigIntegerSizeBits {
		panic(errTooBigInteger)
	} else if sz == MaxBigIntegerSizeBits {
		if value.Sign() == 1 || value.TrailingZeroBits() != MaxBigIntegerSizeBits-1 {
			panic(errTooBigInteger)
		}
	}
	return &BigInteger{
		value: value,
	}
}

// Bytes converts i to a slice of bytes.
func (i *BigInteger) Bytes() []byte {
	return bigint.ToBytes(i.value)
}

// TryBool implements Item interface.
func (i *BigInteger) TryBool() (bool, error) {
	return i.value.Sign() != 0, nil
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

// TryBool implements Item interface.
func (i *Bool) TryBool() (bool, error) { return i.value, nil }

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

// TryBool implements Item interface.
func (i *ByteArray) TryBool() (bool, error) {
	if len(i.value) > MaxBigIntegerSizeBits/8 {
		return false, errTooBigInteger
	}
	for _, b := range i.value {
		if b != 0 {
			return true, nil
		}
	}
	return false, nil
}

// TryBytes implements Item interface.
func (i *ByteArray) TryBytes() ([]byte, error) {
	return i.value, nil
}

// TryInteger implements Item interface.
func (i *ByteArray) TryInteger() (*big.Int, error) {
	if len(i.value) > MaxBigIntegerSizeBits/8 {
		return nil, errTooBigInteger
	}
	return bigint.FromBytes(i.value), nil
}

// Equals implements Item interface.
func (i *ByteArray) Equals(s Item) bool {
	if len(i.value) > MaxByteArrayComparableSize {
		panic(errTooBigComparable)
	}
	if i == s {
		return true
	} else if s == nil {
		return false
	}
	val, ok := s.(*ByteArray)
	if !ok {
		return false
	}
	if len(val.value) > MaxByteArrayComparableSize {
		panic(errTooBigComparable)
	}
	return bytes.Equal(i.value, val.value)
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

// TryBool implements Item interface.
func (i *Array) TryBool() (bool, error) { return true, nil }

// TryBytes implements Item interface.
func (i *Array) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements Item interface.
func (i *Array) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
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

// TryBool implements Item interface.
func (i *Map) TryBool() (bool, error) { return true, nil }

// TryBytes implements Item interface.
func (i *Map) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements Item interface.
func (i *Map) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
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
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(i, typ)
	}
}

// Add adds key-value pair to the map.
func (i *Map) Add(key, value Item) {
	if err := IsValidMapKey(key); err != nil {
		panic(err)
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
func IsValidMapKey(key Item) error {
	switch key.(type) {
	case *Bool, *BigInteger:
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

// TryBool implements Item interface.
func (i *Interop) TryBool() (bool, error) { return true, nil }

// TryBytes implements Item interface.
func (i *Interop) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(i, ByteArrayT)
}

// TryInteger implements Item interface.
func (i *Interop) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
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
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(i, typ)
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

// NewPointerWithHash returns new pointer on the specified position of the
// specified script. It differs from NewPointer in that the script hash is being
// passed explicitly to save on hash calculcation. This hash is then being used
// for pointer comparisons.
func NewPointerWithHash(pos int, script []byte, h util.Uint160) *Pointer {
	return &Pointer{
		pos:    pos,
		script: script,
		hash:   h,
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

// TryBool implements Item interface.
func (p *Pointer) TryBool() (bool, error) {
	return true, nil
}

// TryBytes implements Item interface.
func (p *Pointer) TryBytes() ([]byte, error) {
	return nil, mkInvConversion(p, ByteArrayT)
}

// TryInteger implements Item interface.
func (p *Pointer) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(p, IntegerT)
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
		return NewBool(true), nil
	default:
		return nil, mkInvConversion(p, typ)
	}
}

// ScriptHash returns pointer item hash.
func (p *Pointer) ScriptHash() util.Uint160 {
	return p.hash
}

// Position returns pointer item position.
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

// TryBool implements Item interface.
func (i *Buffer) TryBool() (bool, error) {
	return true, nil
}

// TryBytes implements Item interface.
func (i *Buffer) TryBytes() ([]byte, error) {
	return i.value, nil
}

// TryInteger implements Item interface.
func (i *Buffer) TryInteger() (*big.Int, error) {
	return nil, mkInvConversion(i, IntegerT)
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
		return NewBool(true), nil
	case BufferT:
		return i, nil
	case ByteArrayT:
		val := make([]byte, len(i.value))
		copy(val, i.value)
		return NewByteArray(val), nil
	case IntegerT:
		if len(i.value) > MaxBigIntegerSizeBits/8 {
			return nil, errTooBigInteger
		}
		return NewBigInteger(bigint.FromBytes(i.value)), nil
	default:
		return nil, mkInvConversion(i, typ)
	}
}

// Len returns length of Buffer value.
func (i *Buffer) Len() int {
	return len(i.value)
}

// DeepCopy returns new deep copy of the provided item.
// Values of Interop items are not deeply copied.
// It does preserve duplicates only for non-primitive types.
func DeepCopy(item Item) Item {
	seen := make(map[Item]Item)
	return deepCopy(item, seen)
}

func deepCopy(item Item, seen map[Item]Item) Item {
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
			arr.value[i] = deepCopy(it.value[i], seen)
		}
		return arr
	case *Struct:
		arr := NewStruct(make([]Item, len(it.value)))
		seen[item] = arr
		for i := range it.value {
			arr.value[i] = deepCopy(it.value[i], seen)
		}
		return arr
	case *Map:
		m := NewMap()
		seen[item] = m
		for i := range it.value {
			key := deepCopy(it.value[i].Key, seen)
			value := deepCopy(it.value[i].Value, seen)
			m.Add(key, value)
		}
		return m
	case *BigInteger:
		bi := new(big.Int).SetBytes(it.value.Bytes())
		if it.value.Sign() == -1 {
			bi.Neg(bi)
		}
		return NewBigInteger(bi)
	case *ByteArray:
		val := make([]byte, len(it.value))
		copy(val, it.value)
		return NewByteArray(val)
	case *Buffer:
		val := make([]byte, len(it.value))
		copy(val, it.value)
		return NewBuffer(val)
	case *Bool:
		return NewBool(it.value)
	case *Pointer:
		return NewPointerWithHash(it.pos, it.script, it.hash)
	case *Interop:
		return NewInterop(it.value)
	default:
		return nil
	}
}
