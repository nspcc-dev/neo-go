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
	serializeItemTo(item, w.BinWriter, make(map[StackItem]bool))
	if w.Err != nil {
		return nil, w.Err
	}
	return w.Bytes(), nil
}

func serializeItemTo(item StackItem, w *io.BinWriter, seen map[StackItem]bool) {
	if seen[item] {
		w.Err = errors.New("recursive structures are not supported")
		return
	}

	switch t := item.(type) {
	case *ByteArrayItem:
		w.WriteLE(byte(byteArrayT))
		w.WriteBytes(t.value)
	case *BoolItem:
		w.WriteLE(byte(booleanT))
		w.WriteLE(t.value)
	case *BigIntegerItem:
		w.WriteLE(byte(integerT))
		w.WriteBytes(t.Bytes())
	case *InteropItem:
		w.Err = errors.New("not supported")
	case *ArrayItem, *StructItem:
		seen[item] = true

		_, isArray := t.(*ArrayItem)
		if isArray {
			w.WriteLE(byte(arrayT))
		} else {
			w.WriteLE(byte(structT))
		}

		arr := t.Value().([]StackItem)
		w.WriteVarUint(uint64(len(arr)))
		for i := range arr {
			serializeItemTo(arr[i], w, seen)
		}
	case *MapItem:
		seen[item] = true

		w.WriteLE(byte(mapT))
		w.WriteVarUint(uint64(len(t.value)))
		for k, v := range t.value {
			serializeItemTo(v, w, seen)
			serializeItemTo(makeStackItem(k), w, seen)
		}
	}
}

func deserializeItem(data []byte) (StackItem, error) {
	r := io.NewBinReaderFromBuf(data)
	item := deserializeItemFrom(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return item, nil
}

func deserializeItemFrom(r *io.BinReader) StackItem {
	var t byte
	r.ReadLE(&t)
	if r.Err != nil {
		return nil
	}

	switch stackItemType(t) {
	case byteArrayT:
		data := r.ReadBytes()
		return NewByteArrayItem(data)
	case booleanT:
		var b bool
		r.ReadLE(&b)
		return NewBoolItem(b)
	case integerT:
		data := r.ReadBytes()
		num := new(big.Int).SetBytes(util.ArrayReverse(data))
		return &BigIntegerItem{
			value: num,
		}
	case arrayT, structT:
		size := int(r.ReadVarUint())
		arr := make([]StackItem, size)
		for i := 0; i < size; i++ {
			arr[i] = deserializeItemFrom(r)
		}

		if stackItemType(t) == arrayT {
			return &ArrayItem{value: arr}
		}
		return &StructItem{value: arr}
	case mapT:
		size := int(r.ReadVarUint())
		m := NewMapItem()
		for i := 0; i < size; i++ {
			value := deserializeItemFrom(r)
			key := deserializeItemFrom(r)
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
