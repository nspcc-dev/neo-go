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
	u64 []byte
	u32 []byte
	u16 []byte
	u8  []byte
	Err error
}

// NewBinReaderFromIO makes a BinReader from io.Reader.
func NewBinReaderFromIO(ior io.Reader) *BinReader {
	u64 := make([]byte, 8)
	u32 := u64[:4]
	u16 := u64[:2]
	u8 := u64[:1]
	return &BinReader{r: ior, u64: u64, u32: u32, u16: u16, u8: u8}
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

// ReadU64LE reads a little-endian encoded uint64 value from the underlying
// io.Reader. On read failures it returns zero.
func (r *BinReader) ReadU64LE() uint64 {
	r.ReadBytes(r.u64)
	if r.Err != nil {
		return 0
	}
	return binary.LittleEndian.Uint64(r.u64)
}

// ReadU32LE reads a little-endian encoded uint32 value from the underlying
// io.Reader. On read failures it returns zero.
func (r *BinReader) ReadU32LE() uint32 {
	r.ReadBytes(r.u32)
	if r.Err != nil {
		return 0
	}
	return binary.LittleEndian.Uint32(r.u32)
}

// ReadU16LE reads a little-endian encoded uint16 value from the underlying
// io.Reader. On read failures it returns zero.
func (r *BinReader) ReadU16LE() uint16 {
	r.ReadBytes(r.u16)
	if r.Err != nil {
		return 0
	}
	return binary.LittleEndian.Uint16(r.u16)
}

// ReadU16BE reads a big-endian encoded uint16 value from the underlying
// io.Reader. On read failures it returns zero.
func (r *BinReader) ReadU16BE() uint16 {
	r.ReadBytes(r.u16)
	if r.Err != nil {
		return 0
	}
	return binary.BigEndian.Uint16(r.u16)
}

// ReadByte reads a byte from the underlying io.Reader. On read failures it
// returns zero.
func (r *BinReader) ReadByte() byte {
	r.ReadBytes(r.u8)
	if r.Err != nil {
		return 0
	}
	return r.u8[0]
}

// ReadBool reads a boolean value encoded in a zero/non-zero byte from the
// underlying io.Reader. On read failures it returns false.
func (r *BinReader) ReadBool() bool {
	return r.ReadByte() != 0
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

	var b = r.ReadByte()

	if b == 0xfd {
		return uint64(r.ReadU16LE())
	}
	if b == 0xfe {
		return uint64(r.ReadU32LE())
	}
	if b == 0xff {
		return r.ReadU64LE()
	}

	return uint64(b)
}

// ReadVarBytes reads the next set of bytes from the underlying reader.
// ReadVarUInt() is used to determine how large that slice is
func (r *BinReader) ReadVarBytes() []byte {
	n := r.ReadVarUint()
	b := make([]byte, n)
	r.ReadBytes(b)
	return b
}

// ReadBytes copies fixed-size buffer from the reader to provided slice.
func (r *BinReader) ReadBytes(buf []byte) {
	if r.Err != nil {
		return
	}

	_, r.Err = io.ReadFull(r.r, buf)
}

// ReadString calls ReadVarBytes and casts the results as a string.
func (r *BinReader) ReadString() string {
	b := r.ReadVarBytes()
	return string(b)
}
