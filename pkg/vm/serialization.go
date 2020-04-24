package vm

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// StackItemType represents type of the stack item.
type StackItemType byte

// This block defines all known stack item types.
const (
	AnyT       StackItemType = 0x00
	PointerT   StackItemType = 0x10
	BooleanT   StackItemType = 0x20
	IntegerT   StackItemType = 0x21
	ByteArrayT StackItemType = 0x28
	BufferT    StackItemType = 0x30
	ArrayT     StackItemType = 0x40
	StructT    StackItemType = 0x41
	MapT       StackItemType = 0x48
	InteropT   StackItemType = 0x60
)

// String implements fmt.Stringer interface.
func (t StackItemType) String() string {
	switch t {
	case AnyT:
		return "Any"
	case PointerT:
		return "Pointer"
	case BooleanT:
		return "Boolean"
	case IntegerT:
		return "Integer"
	case ByteArrayT:
		return "ByteArray"
	case BufferT:
		return "Buffer"
	case ArrayT:
		return "Array"
	case StructT:
		return "Struct"
	case MapT:
		return "Map"
	case InteropT:
		return "Interop"
	default:
		return "INVALID"
	}
}

// IsValid checks if s is a well defined stack item type.
func (t StackItemType) IsValid() bool {
	switch t {
	case AnyT, PointerT, BooleanT, IntegerT, ByteArrayT, BufferT, ArrayT, StructT, MapT, InteropT:
		return true
	default:
		return false
	}
}

// SerializeItem encodes given StackItem into the byte slice.
func SerializeItem(item StackItem) ([]byte, error) {
	w := io.NewBufBinWriter()
	EncodeBinaryStackItem(item, w.BinWriter)
	if w.Err != nil {
		return nil, w.Err
	}
	return w.Bytes(), nil
}

// EncodeBinaryStackItem encodes given StackItem into the given BinWriter. It's
// similar to io.Serializable's EncodeBinary, but works with StackItem
// interface.
func EncodeBinaryStackItem(item StackItem, w *io.BinWriter) {
	serializeItemTo(item, w, make(map[StackItem]bool))
}

func serializeItemTo(item StackItem, w *io.BinWriter, seen map[StackItem]bool) {
	if seen[item] {
		w.Err = errors.New("recursive structures can't be serialized")
		return
	}

	switch t := item.(type) {
	case *ByteArrayItem:
		w.WriteBytes([]byte{byte(ByteArrayT)})
		w.WriteVarBytes(t.value)
	case *BoolItem:
		w.WriteBytes([]byte{byte(BooleanT)})
		w.WriteBool(t.value)
	case *BigIntegerItem:
		w.WriteBytes([]byte{byte(IntegerT)})
		w.WriteVarBytes(emit.IntToBytes(t.value))
	case *InteropItem:
		w.Err = errors.New("interop item can't be serialized")
	case *ArrayItem, *StructItem:
		seen[item] = true

		_, isArray := t.(*ArrayItem)
		if isArray {
			w.WriteBytes([]byte{byte(ArrayT)})
		} else {
			w.WriteBytes([]byte{byte(StructT)})
		}

		arr := t.Value().([]StackItem)
		w.WriteVarUint(uint64(len(arr)))
		for i := range arr {
			serializeItemTo(arr[i], w, seen)
		}
	case *MapItem:
		seen[item] = true

		w.WriteBytes([]byte{byte(MapT)})
		w.WriteVarUint(uint64(len(t.value)))
		for i := range t.value {
			serializeItemTo(t.value[i].Key, w, seen)
			serializeItemTo(t.value[i].Value, w, seen)
		}
	}
}

// DeserializeItem decodes StackItem from the given byte slice.
func DeserializeItem(data []byte) (StackItem, error) {
	r := io.NewBinReaderFromBuf(data)
	item := DecodeBinaryStackItem(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return item, nil
}

// DecodeBinaryStackItem decodes previously serialized StackItem from the given
// reader. It's similar to the io.Serializable's DecodeBinary(), but implemented
// as a function because StackItem itself is an interface. Caveat: always check
// reader's error value before using the returned StackItem.
func DecodeBinaryStackItem(r *io.BinReader) StackItem {
	var t = r.ReadB()
	if r.Err != nil {
		return nil
	}

	switch StackItemType(t) {
	case ByteArrayT:
		data := r.ReadVarBytes()
		return NewByteArrayItem(data)
	case BooleanT:
		var b = r.ReadBool()
		return NewBoolItem(b)
	case IntegerT:
		data := r.ReadVarBytes()
		num := emit.BytesToInt(data)
		return &BigIntegerItem{
			value: num,
		}
	case ArrayT, StructT:
		size := int(r.ReadVarUint())
		arr := make([]StackItem, size)
		for i := 0; i < size; i++ {
			arr[i] = DecodeBinaryStackItem(r)
		}

		if StackItemType(t) == ArrayT {
			return &ArrayItem{value: arr}
		}
		return &StructItem{value: arr}
	case MapT:
		size := int(r.ReadVarUint())
		m := NewMapItem()
		for i := 0; i < size; i++ {
			key := DecodeBinaryStackItem(r)
			value := DecodeBinaryStackItem(r)
			if r.Err != nil {
				break
			}
			m.Add(key, value)
		}
		return m
	default:
		r.Err = errors.New("unknown type")
		return nil
	}
}
