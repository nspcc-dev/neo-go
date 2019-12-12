package vm

import (
	"errors"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

type stackItemType byte

const (
	byteArrayT stackItemType = 0x00
	booleanT   stackItemType = 0x01
	integerT   stackItemType = 0x02
	arrayT     stackItemType = 0x80
	structT    stackItemType = 0x81
	mapT       stackItemType = 0x82
)

func serializeItem(item StackItem) ([]byte, error) {
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
		w.Err = errors.New("recursive structures are not supported")
		return
	}

	switch t := item.(type) {
	case *ByteArrayItem:
		w.WriteBytes([]byte{byte(byteArrayT)})
		w.WriteVarBytes(t.value)
	case *BoolItem:
		w.WriteBytes([]byte{byte(booleanT)})
		w.WriteBool(t.value)
	case *BigIntegerItem:
		w.WriteBytes([]byte{byte(integerT)})
		w.WriteVarBytes(t.Bytes())
	case *InteropItem:
		w.Err = errors.New("not supported")
	case *ArrayItem, *StructItem:
		seen[item] = true

		_, isArray := t.(*ArrayItem)
		if isArray {
			w.WriteBytes([]byte{byte(arrayT)})
		} else {
			w.WriteBytes([]byte{byte(structT)})
		}

		arr := t.Value().([]StackItem)
		w.WriteVarUint(uint64(len(arr)))
		for i := range arr {
			serializeItemTo(arr[i], w, seen)
		}
	case *MapItem:
		seen[item] = true

		w.WriteBytes([]byte{byte(mapT)})
		w.WriteVarUint(uint64(len(t.value)))
		for k, v := range t.value {
			serializeItemTo(v, w, seen)
			serializeItemTo(makeStackItem(k), w, seen)
		}
	}
}

func deserializeItem(data []byte) (StackItem, error) {
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

	switch stackItemType(t) {
	case byteArrayT:
		data := r.ReadVarBytes()
		return NewByteArrayItem(data)
	case booleanT:
		var b = r.ReadBool()
		return NewBoolItem(b)
	case integerT:
		data := r.ReadVarBytes()
		num := new(big.Int).SetBytes(util.ArrayReverse(data))
		return &BigIntegerItem{
			value: num,
		}
	case arrayT, structT:
		size := int(r.ReadVarUint())
		arr := make([]StackItem, size)
		for i := 0; i < size; i++ {
			arr[i] = DecodeBinaryStackItem(r)
		}

		if stackItemType(t) == arrayT {
			return &ArrayItem{value: arr}
		}
		return &StructItem{value: arr}
	case mapT:
		size := int(r.ReadVarUint())
		m := NewMapItem()
		for i := 0; i < size; i++ {
			value := DecodeBinaryStackItem(r)
			key := DecodeBinaryStackItem(r)
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
