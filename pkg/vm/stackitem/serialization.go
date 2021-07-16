package stackitem

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// ErrRecursive is returned on attempts to serialize some recursive stack item
// (like array including an item with reference to the same array).
var ErrRecursive = errors.New("recursive item")

// ErrUnserializable is returned on attempt to serialize some item that can't
// be serialized (like Interop item or Pointer).
var ErrUnserializable = errors.New("unserializable")

// serContext is an internal serialization context.
type serContext struct {
	uv           [9]byte
	data         []byte
	allowInvalid bool
	seen         map[Item]sliceNoPointer
}

// Serialize encodes given Item into the byte slice.
func Serialize(item Item) ([]byte, error) {
	sc := serContext{
		allowInvalid: false,
		seen:         make(map[Item]sliceNoPointer),
	}
	err := sc.serialize(item)
	if err != nil {
		return nil, err
	}
	return sc.data, nil
}

// EncodeBinary encodes given Item into the given BinWriter. It's
// similar to io.Serializable's EncodeBinary, but works with Item
// interface.
func EncodeBinary(item Item, w *io.BinWriter) {
	data, err := Serialize(item)
	if err != nil {
		w.Err = err
		return
	}
	w.WriteBytes(data)
}

// EncodeBinaryProtected encodes given Item into the given BinWriter. It's
// similar to EncodeBinary but allows to encode interop items (only type,
// value is lost) and doesn't return any errors in w, instead if error
// (like recursive array) is encountered it just writes special InvalidT
// type of element to w.
func EncodeBinaryProtected(item Item, w *io.BinWriter) {
	sc := serContext{
		allowInvalid: true,
		seen:         make(map[Item]sliceNoPointer),
	}
	err := sc.serialize(item)
	if err != nil {
		w.WriteBytes([]byte{byte(InvalidT)})
		return
	}
	w.WriteBytes(sc.data)
}

func (w *serContext) serialize(item Item) error {
	if v, ok := w.seen[item]; ok {
		if v.start == v.end {
			return ErrRecursive
		}
		if len(w.data)+v.end-v.start > MaxSize {
			return ErrTooBig
		}
		w.data = append(w.data, w.data[v.start:v.end]...)
		return nil
	}

	start := len(w.data)
	switch t := item.(type) {
	case *ByteArray:
		w.data = append(w.data, byte(ByteArrayT))
		data := t.Value().([]byte)
		w.appendVarUint(uint64(len(data)))
		w.data = append(w.data, data...)
	case *Buffer:
		w.data = append(w.data, byte(BufferT))
		data := t.Value().([]byte)
		w.appendVarUint(uint64(len(data)))
		w.data = append(w.data, data...)
	case *Bool:
		w.data = append(w.data, byte(BooleanT))
		if t.Value().(bool) {
			w.data = append(w.data, 1)
		} else {
			w.data = append(w.data, 0)
		}
	case *BigInteger:
		w.data = append(w.data, byte(IntegerT))
		data := bigint.ToBytes(t.Value().(*big.Int))
		w.appendVarUint(uint64(len(data)))
		w.data = append(w.data, data...)
	case *Interop:
		if w.allowInvalid {
			w.data = append(w.data, byte(InteropT))
		} else {
			return fmt.Errorf("%w: Interop", ErrUnserializable)
		}
	case *Array, *Struct:
		w.seen[item] = sliceNoPointer{}

		_, isArray := t.(*Array)
		if isArray {
			w.data = append(w.data, byte(ArrayT))
		} else {
			w.data = append(w.data, byte(StructT))
		}

		arr := t.Value().([]Item)
		w.appendVarUint(uint64(len(arr)))
		for i := range arr {
			if err := w.serialize(arr[i]); err != nil {
				return err
			}
		}
		w.seen[item] = sliceNoPointer{start, len(w.data)}
	case *Map:
		w.seen[item] = sliceNoPointer{}

		elems := t.Value().([]MapElement)
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
		w.seen[item] = sliceNoPointer{start, len(w.data)}
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

func (w *serContext) appendVarUint(val uint64) {
	n := io.PutVarUint(w.uv[:], val)
	w.data = append(w.data, w.uv[:n]...)
}

// Deserialize decodes Item from the given byte slice.
func Deserialize(data []byte) (Item, error) {
	r := io.NewBinReaderFromBuf(data)
	item := DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return item, nil
}

// DecodeBinary decodes previously serialized Item from the given
// reader. It's similar to the io.Serializable's DecodeBinary(), but implemented
// as a function because Item itself is an interface. Caveat: always check
// reader's error value before using the returned Item.
func DecodeBinary(r *io.BinReader) Item {
	return decodeBinary(r, false)
}

// DecodeBinaryProtected is similar to DecodeBinary but allows Interop and
// Invalid values to be present (making it symmetric to EncodeBinaryProtected).
func DecodeBinaryProtected(r *io.BinReader) Item {
	return decodeBinary(r, true)
}

func decodeBinary(r *io.BinReader, allowInvalid bool) Item {
	var t = Type(r.ReadB())
	if r.Err != nil {
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
		arr := make([]Item, size)
		for i := 0; i < size; i++ {
			arr[i] = decodeBinary(r, allowInvalid)
		}

		if t == ArrayT {
			return NewArray(arr)
		}
		return NewStruct(arr)
	case MapT:
		size := int(r.ReadVarUint())
		m := NewMap()
		for i := 0; i < size; i++ {
			key := decodeBinary(r, allowInvalid)
			value := decodeBinary(r, allowInvalid)
			if r.Err != nil {
				break
			}
			m.Add(key, value)
		}
		return m
	case AnyT:
		return Null{}
	case InteropT:
		if allowInvalid {
			return NewInterop(nil)
		}
		fallthrough
	default:
		if t == InvalidT && allowInvalid {
			return nil
		}
		r.Err = fmt.Errorf("%w: %v", ErrInvalidType, t)
		return nil
	}
}
