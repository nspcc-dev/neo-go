package stackitem

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MaxDeserialized is the maximum number one deserialized item can contain
// (including itself).
const MaxDeserialized = 2048

// MaxSerialized is the maximum number one serialized item can contain
// (including itself).
const MaxSerialized = MaxDeserialized

// typicalNumOfItems is the number of items covering most serialization needs.
// It's a hint used for map creation, so it does not limit anything, it's just
// a microoptimization to avoid excessive reallocations. Most of the serialized
// items are structs, so there is at least one of them.
const typicalNumOfItems = 4

// ErrRecursive is returned upon an attempt to serialize some recursive stack item
// (like an array including an item with the reference to the same array).
var ErrRecursive = errors.New("recursive item")

// ErrUnserializable is returned upon an attempt to serialize some item that can't
// be serialized (like Interop item or Pointer).
var ErrUnserializable = errors.New("unserializable")

// SerializationContext is a serialization context.
type SerializationContext struct {
	uv           [9]byte
	data         []byte
	allowInvalid bool
	limit        int
	seen         map[Item]sliceNoPointer
}

// deserContext is an internal deserialization context.
type deserContext struct {
	*io.BinReader
	allowInvalid bool
	limit        int
}

// Serialize encodes the given Item into a byte slice.
func Serialize(item Item) ([]byte, error) {
	return SerializeLimited(item, MaxSerialized)
}

// SerializeLimited encodes the given Item into a byte slice using custom
// limit to restrict the maximum serialized number of elements.
func SerializeLimited(item Item, limit int) ([]byte, error) {
	sc := SerializationContext{
		allowInvalid: false,
		limit:        MaxSerialized,
		seen:         make(map[Item]sliceNoPointer, typicalNumOfItems),
	}
	if limit > 0 {
		sc.limit = limit
	}
	err := sc.serialize(item)
	if err != nil {
		return nil, err
	}
	return sc.data, nil
}

// EncodeBinary encodes the given Item into the given BinWriter. It's
// similar to io.Serializable's EncodeBinary but works with Item
// interface.
func EncodeBinary(item Item, w *io.BinWriter) {
	data, err := Serialize(item)
	if err != nil {
		w.Err = err
		return
	}
	w.WriteBytes(data)
}

// EncodeBinaryProtected encodes the given Item into the given BinWriter. It's
// similar to EncodeBinary but allows to encode interop items (only type,
// value is lost) and doesn't return any errors in the w. Instead, if an error
// (like recursive array) is encountered, it just writes the special InvalidT
// type of an element to the w.
func EncodeBinaryProtected(item Item, w *io.BinWriter) {
	sc := SerializationContext{
		allowInvalid: true,
		limit:        MaxSerialized,
		seen:         make(map[Item]sliceNoPointer, typicalNumOfItems),
	}
	err := sc.serialize(item)
	if err != nil {
		w.WriteBytes([]byte{byte(InvalidT)})
		return
	}
	w.WriteBytes(sc.data)
}

func (w *SerializationContext) writeArray(item Item, arr []Item, start int) error {
	w.seen[item] = sliceNoPointer{}
	limit := w.limit
	w.appendVarUint(uint64(len(arr)))
	for i := range arr {
		if err := w.serialize(arr[i]); err != nil {
			return err
		}
	}
	w.seen[item] = sliceNoPointer{start, len(w.data), limit - w.limit + 1} // number of items including the array itself.
	return nil
}

// NewSerializationContext returns reusable stack item serialization context.
func NewSerializationContext() *SerializationContext {
	return &SerializationContext{
		limit: MaxSerialized,
		seen:  make(map[Item]sliceNoPointer, typicalNumOfItems),
	}
}

// Serialize returns flat slice of bytes with the given item. The process can be protected
// from bad elements if appropriate flag is given (otherwise an error is returned on
// encountering any of them). The buffer returned is only valid until the call to Serialize.
// The number of serialized items is restricted with MaxSerialized.
func (w *SerializationContext) Serialize(item Item, protected bool) ([]byte, error) {
	w.allowInvalid = protected
	w.limit = MaxSerialized
	if w.data != nil {
		w.data = w.data[:0]
	}
	clear(w.seen)
	err := w.serialize(item)
	if err != nil && protected {
		if w.data == nil {
			w.data = make([]byte, 0, 1)
		}
		w.data = append(w.data[:0], byte(InvalidT))
		err = nil
	}
	return w.data, err
}

func (w *SerializationContext) serialize(item Item) error {
	if v, ok := w.seen[item]; ok {
		if v.start == v.end {
			return ErrRecursive
		}
		if len(w.data)+v.end-v.start > MaxSize {
			return ErrTooBig
		}
		w.limit -= v.itemsCount
		if w.limit < 0 {
			return errTooBigElements
		}
		w.data = append(w.data, w.data[v.start:v.end]...)
		return nil
	}
	w.limit--
	if w.limit < 0 {
		return errTooBigElements
	}
	start := len(w.data)
	switch t := item.(type) {
	case *ByteArray:
		w.data = append(w.data, byte(ByteArrayT))
		w.appendVarUint(uint64(len(*t)))
		w.data = append(w.data, *t...)
	case *Buffer:
		w.data = append(w.data, byte(BufferT))
		w.appendVarUint(uint64(len(*t)))
		w.data = append(w.data, *t...)
	case Bool:
		w.data = append(w.data, byte(BooleanT))
		if t {
			w.data = append(w.data, 1)
		} else {
			w.data = append(w.data, 0)
		}
	case *BigInteger:
		w.data = append(w.data, byte(IntegerT))
		ln := len(w.data)
		w.data = append(w.data, 0)
		data := bigint.ToPreallocatedBytes((*big.Int)(t), w.data[len(w.data):])
		w.data[ln] = byte(len(data))
		w.data = append(w.data, data...)
	case *Interop:
		if w.allowInvalid {
			w.data = append(w.data, byte(InteropT))
		} else {
			return fmt.Errorf("%w: Interop", ErrUnserializable)
		}
	case *Pointer:
		if w.allowInvalid {
			w.data = append(w.data, byte(PointerT))
			w.appendVarUint(uint64(t.pos))
		} else {
			return fmt.Errorf("%w: Pointer", ErrUnserializable)
		}
	case *Array:
		w.data = append(w.data, byte(ArrayT))
		if err := w.writeArray(item, t.value, start); err != nil {
			return err
		}
	case *Struct:
		w.data = append(w.data, byte(StructT))
		if err := w.writeArray(item, t.value, start); err != nil {
			return err
		}
	case *Map:
		w.seen[item] = sliceNoPointer{}
		limit := w.limit

		elems := t.value
		w.data = append(w.data, byte(MapT))
		w.appendVarUint(uint64(len(elems)))
		for i := range elems {
			if err := w.serialize(elems[i].Key); err != nil {
				return err
			}
			if err := w.serialize(elems[i].Value); err != nil {
				return err
			}
		}
		w.seen[item] = sliceNoPointer{start, len(w.data), limit - w.limit + 1} // number of items including Map itself.
	case Null:
		w.data = append(w.data, byte(AnyT))
	case nil:
		if w.allowInvalid {
			w.data = append(w.data, byte(InvalidT))
		} else {
			return fmt.Errorf("%w: nil", ErrUnserializable)
		}
	}

	if len(w.data) > MaxSize {
		return errTooBigSize
	}
	return nil
}

func (w *SerializationContext) appendVarUint(val uint64) {
	n := io.PutVarUint(w.uv[:], val)
	w.data = append(w.data, w.uv[:n]...)
}

// Deserialize decodes the Item from the given byte slice.
func Deserialize(data []byte) (Item, error) {
	r := io.NewBinReaderFromBuf(data)
	item := DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return item, nil
}

// DeserializeLimited returns Item deserialized from the given byte slice. limit
// restricts the maximum number of items deserialized item can contain (including
// itself). The default limit of MaxDeserialized is used if non-positive limit is
// specified.
func DeserializeLimited(data []byte, limit int) (Item, error) {
	r := io.NewBinReaderFromBuf(data)
	dc := deserContext{
		BinReader:    r,
		allowInvalid: false,
		limit:        MaxDeserialized,
	}
	if limit > 0 {
		dc.limit = limit
	}
	item := dc.decodeBinary()
	if r.Err != nil {
		return nil, r.Err
	}
	return item, nil
}

// DecodeBinary decodes the previously serialized Item from the given
// reader. It's similar to the io.Serializable's DecodeBinary() but implemented
// as a function because Item itself is an interface. Caveat: always check
// reader's error value before using the returned Item.
func DecodeBinary(r *io.BinReader) Item {
	dc := deserContext{
		BinReader:    r,
		allowInvalid: false,
		limit:        MaxDeserialized,
	}
	return dc.decodeBinary()
}

// DecodeBinaryProtected is similar to DecodeBinary but allows Interop and
// Invalid values to be present (making it symmetric to EncodeBinaryProtected).
func DecodeBinaryProtected(r *io.BinReader) Item {
	dc := deserContext{
		BinReader:    r,
		allowInvalid: true,
		limit:        MaxDeserialized,
	}
	return dc.decodeBinary()
}

func (r *deserContext) decodeBinary() Item {
	var t = Type(r.ReadB())
	if r.Err != nil {
		return nil
	}

	r.limit--
	if r.limit < 0 {
		r.Err = errTooBigElements
		return nil
	}
	switch t {
	case ByteArrayT, BufferT:
		data := r.ReadVarBytes(MaxSize)
		if t == ByteArrayT {
			return NewByteArray(data)
		}
		return NewBuffer(data)
	case BooleanT:
		var b = r.ReadBool()
		return NewBool(b)
	case IntegerT:
		data := r.ReadVarBytes(bigint.MaxBytesLen)
		num := bigint.FromBytes(data)
		return NewBigInteger(num)
	case ArrayT, StructT:
		size := int(r.ReadVarUint())
		if size > r.limit {
			r.Err = errTooBigElements
			return nil
		}
		arr := make([]Item, size)
		for i := 0; i < size; i++ {
			arr[i] = r.decodeBinary()
		}

		if t == ArrayT {
			return NewArray(arr)
		}
		return NewStruct(arr)
	case MapT:
		size := int(r.ReadVarUint())
		if size > r.limit/2 {
			r.Err = errTooBigElements
			return nil
		}
		m := NewMap()
		for i := 0; i < size; i++ {
			key := r.decodeBinary()
			value := r.decodeBinary()
			if r.Err != nil {
				break
			}
			m.Add(key, value)
		}
		return m
	case AnyT:
		return Null{}
	case InteropT:
		if r.allowInvalid {
			return NewInterop(nil)
		}
		fallthrough
	case PointerT:
		if r.allowInvalid {
			pos := int(r.ReadVarUint())
			return NewPointerWithHash(pos, nil, util.Uint160{})
		}
		fallthrough
	default:
		if t == InvalidT && r.allowInvalid {
			return nil
		}
		r.Err = fmt.Errorf("%w: %v", ErrInvalidType, t)
		return nil
	}
}

// SerializeConvertible serializes Convertible into a slice of bytes.
func SerializeConvertible(conv Convertible) ([]byte, error) {
	item, err := conv.ToStackItem()
	if err != nil {
		return nil, err
	}
	return Serialize(item)
}

// DeserializeConvertible deserializes Convertible from a slice of bytes.
func DeserializeConvertible(data []byte, conv Convertible) error {
	item, err := Deserialize(data)
	if err != nil {
		return err
	}
	return conv.FromStackItem(item)
}
