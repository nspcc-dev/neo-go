package stackitem

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// serContext is an internal serialization context.
type serContext struct {
	*io.BinWriter
	buf          *io.BufBinWriter
	allowInvalid bool
	seen         map[Item]bool
}

// Serialize encodes given Item into the byte slice.
func Serialize(item Item) ([]byte, error) {
	w := io.NewBufBinWriter()
	sc := serContext{
		BinWriter:    w.BinWriter,
		buf:          w,
		allowInvalid: false,
		seen:         make(map[Item]bool),
	}
	sc.serialize(item)
	if w.Err != nil {
		return nil, w.Err
	}
	return w.Bytes(), nil
}

// EncodeBinary encodes given Item into the given BinWriter. It's
// similar to io.Serializable's EncodeBinary, but works with Item
// interface.
func EncodeBinary(item Item, w *io.BinWriter) {
	sc := serContext{
		BinWriter:    w,
		allowInvalid: false,
		seen:         make(map[Item]bool),
	}
	sc.serialize(item)
}

// EncodeBinaryProtected encodes given Item into the given BinWriter. It's
// similar to EncodeBinary but allows to encode interop items (only type,
// value is lost) and doesn't return any errors in w, instead if error
// (like recursive array) is encountered it just writes special InvalidT
// type of element to w.
func EncodeBinaryProtected(item Item, w *io.BinWriter) {
	bw := io.NewBufBinWriter()
	sc := serContext{
		BinWriter:    bw.BinWriter,
		buf:          bw,
		allowInvalid: true,
		seen:         make(map[Item]bool),
	}
	sc.serialize(item)
	if bw.Err != nil {
		w.WriteBytes([]byte{byte(InvalidT)})
		return
	}
	w.WriteBytes(bw.Bytes())
}

func (w *serContext) serialize(item Item) {
	if w.Err != nil {
		return
	}
	if w.seen[item] {
		w.Err = errors.New("recursive structures can't be serialized")
		return
	}

	switch t := item.(type) {
	case *ByteArray:
		w.WriteBytes([]byte{byte(ByteArrayT)})
		w.WriteVarBytes(t.Value().([]byte))
	case *Buffer:
		w.WriteBytes([]byte{byte(BufferT)})
		w.WriteVarBytes(t.Value().([]byte))
	case *Bool:
		w.WriteBytes([]byte{byte(BooleanT)})
		w.WriteBool(t.Value().(bool))
	case *BigInteger:
		w.WriteBytes([]byte{byte(IntegerT)})
		w.WriteVarBytes(bigint.ToBytes(t.Value().(*big.Int)))
	case *Interop:
		if w.allowInvalid {
			w.WriteBytes([]byte{byte(InteropT)})
		} else {
			w.Err = errors.New("interop item can't be serialized")
		}
	case *Array, *Struct:
		w.seen[item] = true

		_, isArray := t.(*Array)
		if isArray {
			w.WriteBytes([]byte{byte(ArrayT)})
		} else {
			w.WriteBytes([]byte{byte(StructT)})
		}

		arr := t.Value().([]Item)
		w.WriteVarUint(uint64(len(arr)))
		for i := range arr {
			w.serialize(arr[i])
		}
		delete(w.seen, item)
	case *Map:
		w.seen[item] = true

		w.WriteBytes([]byte{byte(MapT)})
		w.WriteVarUint(uint64(len(t.Value().([]MapElement))))
		for i := range t.Value().([]MapElement) {
			w.serialize(t.Value().([]MapElement)[i].Key)
			w.serialize(t.Value().([]MapElement)[i].Value)
		}
		delete(w.seen, item)
	case Null:
		w.WriteB(byte(AnyT))
	case nil:
		if w.allowInvalid {
			w.WriteBytes([]byte{byte(InvalidT)})
		} else {
			w.Err = errors.New("invalid stack item")
		}
	}

	if w.Err == nil && w.buf != nil && w.buf.Len() > MaxSize {
		w.Err = errors.New("too big item")
	}
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
			arr[i] = DecodeBinary(r)
		}

		if t == ArrayT {
			return NewArray(arr)
		}
		return NewStruct(arr)
	case MapT:
		size := int(r.ReadVarUint())
		m := NewMap()
		for i := 0; i < size; i++ {
			key := DecodeBinary(r)
			value := DecodeBinary(r)
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
		r.Err = fmt.Errorf("unknown type: %v", t)
		return nil
	}
}
