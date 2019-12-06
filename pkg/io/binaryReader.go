package io

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
)

// maxArraySize is a maximums size of an array which can be decoded.
// It is taken from https://github.com/neo-project/neo/blob/master/neo/IO/Helper.cs#L130
const maxArraySize = 0x1000000

// BinReader is a convenient wrapper around a io.Reader and err object.
// Used to simplify error handling when reading into a struct with many fields.
type BinReader struct {
	r   io.Reader
	Err error
}

// NewBinReaderFromIO makes a BinReader from io.Reader.
func NewBinReaderFromIO(ior io.Reader) *BinReader {
	return &BinReader{r: ior}
}

// NewBinReaderFromBuf makes a BinReader from byte buffer.
func NewBinReaderFromBuf(b []byte) *BinReader {
	r := bytes.NewReader(b)
	return NewBinReaderFromIO(r)
}

// ReadLE reads from the underlying io.Reader
// into the interface v in little-endian format.
func (r *BinReader) ReadLE(v interface{}) {
	if r.Err != nil {
		return
	}
	r.Err = binary.Read(r.r, binary.LittleEndian, v)
}

// ReadArray reads array into value which must be
// a pointer to a slice.
func (r *BinReader) ReadArray(t interface{}, maxSize ...int) {
	value := reflect.ValueOf(t)
	if value.Kind() != reflect.Ptr || value.Elem().Kind() != reflect.Slice {
		panic(value.Type().String() + " is not a pointer to a slice")
	}

	if r.Err != nil {
		return
	}

	sliceType := value.Elem().Type()
	elemType := sliceType.Elem()
	isPtr := elemType.Kind() == reflect.Ptr

	ms := maxArraySize
	if len(maxSize) != 0 {
		ms = maxSize[0]
	}

	lu := r.ReadVarUint()
	if lu > uint64(ms) {
		r.Err = fmt.Errorf("array is too big (%d)", lu)
		return
	}

	l := int(lu)
	arr := reflect.MakeSlice(sliceType, l, l)

	for i := 0; i < l; i++ {
		var elem reflect.Value
		if isPtr {
			elem = reflect.New(elemType.Elem())
			arr.Index(i).Set(elem)
		} else {
			elem = arr.Index(i).Addr()
		}

		el, ok := elem.Interface().(decodable)
		if !ok {
			panic(elemType.String() + "is not decodable")
		}

		el.DecodeBinary(r)
	}

	value.Elem().Set(arr)
}

// ReadBE reads from the underlying io.Reader
// into the interface v in big-endian format.
func (r *BinReader) ReadBE(v interface{}) {
	if r.Err != nil {
		return
	}
	r.Err = binary.Read(r.r, binary.BigEndian, v)
}

// ReadVarUint reads a variable-length-encoded integer from the
// underlying reader.
func (r *BinReader) ReadVarUint() uint64 {
	if r.Err != nil {
		return 0
	}

	var b uint8
	r.Err = binary.Read(r.r, binary.LittleEndian, &b)

	if b == 0xfd {
		var v uint16
		r.Err = binary.Read(r.r, binary.LittleEndian, &v)
		return uint64(v)
	}
	if b == 0xfe {
		var v uint32
		r.Err = binary.Read(r.r, binary.LittleEndian, &v)
		return uint64(v)
	}
	if b == 0xff {
		var v uint64
		r.Err = binary.Read(r.r, binary.LittleEndian, &v)
		return v
	}

	return uint64(b)
}

// ReadVarBytes reads the next set of bytes from the underlying reader.
// ReadVarUInt() is used to determine how large that slice is
func (r *BinReader) ReadVarBytes() []byte {
	n := r.ReadVarUint()
	b := make([]byte, n)
	r.ReadLE(b)
	return b
}

// ReadString calls ReadVarBytes and casts the results as a string.
func (r *BinReader) ReadString() string {
	b := r.ReadVarBytes()
	return string(b)
}
