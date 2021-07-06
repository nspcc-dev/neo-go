package stackitem

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// SerializeItem encodes given Item into the byte slice.
func SerializeItem(item Item) ([]byte, error) {
	w := io.NewBufBinWriter()
	EncodeBinaryStackItem(item, w.BinWriter)
	if w.Err != nil {
		return nil, w.Err
	}
	return w.Bytes(), nil
}

// EncodeBinaryStackItem encodes given Item into the given BinWriter. It's
// similar to io.Serializable's EncodeBinary, but works with Item
// interface.
func EncodeBinaryStackItem(item Item, w *io.BinWriter) {
	serializeItemTo(item, w, false, make(map[Item]bool))
}

// EncodeBinaryStackItemAppExec encodes given Item into the given BinWriter. It's
// similar to EncodeBinaryStackItem but allows to encode interop (only type, value is lost).
func EncodeBinaryStackItemAppExec(item Item, w *io.BinWriter) {
	bw := io.NewBufBinWriter()
	serializeItemTo(item, bw.BinWriter, true, make(map[Item]bool))
	if bw.Err != nil {
		w.WriteBytes([]byte{byte(InvalidT)})
		return
	}
	w.WriteBytes(bw.Bytes())
}

func serializeItemTo(item Item, w *io.BinWriter, allowInvalid bool, seen map[Item]bool) {
	if w.Err != nil {
		return
	}
	if seen[item] {
		w.Err = errors.New("recursive structures can't be serialized")
		return
	}
	if item == nil && allowInvalid {
		w.WriteBytes([]byte{byte(InvalidT)})
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
		if allowInvalid {
			w.WriteBytes([]byte{byte(InteropT)})
			return
		}
		w.Err = errors.New("interop item can't be serialized")
	case *Array, *Struct:
		seen[item] = true

		_, isArray := t.(*Array)
		if isArray {
			w.WriteBytes([]byte{byte(ArrayT)})
		} else {
			w.WriteBytes([]byte{byte(StructT)})
		}

		arr := t.Value().([]Item)
		w.WriteVarUint(uint64(len(arr)))
		for i := range arr {
			serializeItemTo(arr[i], w, allowInvalid, seen)
		}
		delete(seen, item)
	case *Map:
		seen[item] = true

		w.WriteBytes([]byte{byte(MapT)})
		w.WriteVarUint(uint64(len(t.Value().([]MapElement))))
		for i := range t.Value().([]MapElement) {
			serializeItemTo(t.Value().([]MapElement)[i].Key, w, allowInvalid, seen)
			serializeItemTo(t.Value().([]MapElement)[i].Value, w, allowInvalid, seen)
		}
		delete(seen, item)
	case Null:
		w.WriteB(byte(AnyT))
	}
}

// DeserializeItem decodes Item from the given byte slice.
func DeserializeItem(data []byte) (Item, error) {
	r := io.NewBinReaderFromBuf(data)
	item := DecodeBinaryStackItem(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return item, nil
}

// DecodeBinaryStackItem decodes previously serialized Item from the given
// reader. It's similar to the io.Serializable's DecodeBinary(), but implemented
// as a function because Item itself is an interface. Caveat: always check
// reader's error value before using the returned Item.
func DecodeBinaryStackItem(r *io.BinReader) Item {
	return decodeBinaryStackItem(r, false)
}

// DecodeBinaryStackItemAppExec is similar to DecodeBinaryStackItem
// but allows Interop values to be present.
func DecodeBinaryStackItemAppExec(r *io.BinReader) Item {
	return decodeBinaryStackItem(r, true)
}

func decodeBinaryStackItem(r *io.BinReader, allowInvalid bool) Item {
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
			arr[i] = DecodeBinaryStackItem(r)
		}

		if t == ArrayT {
			return NewArray(arr)
		}
		return NewStruct(arr)
	case MapT:
		size := int(r.ReadVarUint())
		m := NewMap()
		for i := 0; i < size; i++ {
			key := DecodeBinaryStackItem(r)
			value := DecodeBinaryStackItem(r)
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
