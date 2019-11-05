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
	serializeItemTo(item, w.BinWriter)
	if w.Err != nil {
		return nil, w.Err
	}
	return w.Bytes(), nil
}

func serializeItemTo(item StackItem, w *io.BinWriter) {
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
	case *ArrayItem:
		w.Err = errors.New("not implemented")
	case *StructItem:
		w.Err = errors.New("not implemented")
	case *MapItem:
		w.Err = errors.New("not implemented")
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
	case arrayT:
		r.Err = errors.New("not implemented")
		return nil
	case structT:
		r.Err = errors.New("not implemented")
		return nil
	case mapT:
		r.Err = errors.New("not implemented")
		return nil
	default:
		r.Err = errors.New("unknown type")
		return nil
	}
}
