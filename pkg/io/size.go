package io

import (
	"fmt"
	"reflect"
)

// This structure is used to calculate the wire size of the serializable
// structure. It's an io.Writer that doesn't do any real writes, but instead
// just counts the number of bytes to be written.
type counterWriter struct {
	counter int
	err     error
}

var _ BinaryWriter = (*counterWriter)(nil)

func (cw *counterWriter) WriteU64LE(uint64) {
	cw.counter += 8
}

func (cw *counterWriter) WriteU32LE(uint32) {
	cw.counter += 4
}

func (cw *counterWriter) WriteU16LE(u16 uint16) {
	cw.counter += 2
}

func (cw *counterWriter) WriteU16BE(u16 uint16) {
	cw.counter += 2
}

func (cw *counterWriter) WriteB(u8 byte) {
	cw.counter++
}

func (cw *counterWriter) WriteBool(b bool) {
	cw.counter++
}

func (cw *counterWriter) WriteArray(arr interface{}) {
	writeArray(cw, arr)
}

func (cw *counterWriter) WriteVarUint(val uint64) {
	cw.counter += GetVarSize(val)
}

func (cw *counterWriter) WriteBytes(b []byte) {
	cw.counter += len(b)
}

func (cw *counterWriter) WriteVarBytes(b []byte) {
	cw.WriteVarUint(uint64(len(b)))
	cw.counter += len(b)
}

func (cw *counterWriter) WriteString(s string) {
	cw.WriteVarUint(uint64(len(s)))
	cw.counter += len(s)
}

func (cw *counterWriter) Error() error {
	return cw.err
}

func (cw *counterWriter) SetError(err error) {
	cw.err = err
}

// getVarIntSize returns the size in number of bytes of a variable integer.
// (reference: GetVarSize(int value),  https://github.com/neo-project/neo/blob/master/neo/IO/Helper.cs)
func getVarIntSize(value int) int {
	var size uintptr

	if value < 0xFD {
		size = 1 // unit8
	} else if value <= 0xFFFF {
		size = 3 // byte + uint16
	} else {
		size = 5 // byte + uint32
	}
	return int(size)
}

// GetVarSize returns the number of bytes in a serialized variable. It supports ints/uints (estimating
// them using variable-length encoding that is used in NEO), strings, pointers to Serializable structures,
// slices and arrays of ints/uints or Serializable structures. It's similar to GetVarSize<T>(this T[] value)
// used in C#, but differs in that it also supports things like Uint160 or Uint256.
func GetVarSize(value interface{}) int {
	switch v := value.(type) {
	case int:
		return getVarIntSize(int(v))
	case int8:
		return getVarIntSize(int(v))
	case int16:
		return getVarIntSize(int(v))
	case int32:
		return getVarIntSize(int(v))
	case int64:
		return getVarIntSize(int(v))
	case uint:
		return getVarIntSize(int(v))
	case uint8:
		return getVarIntSize(int(v))
	case uint16:
		return getVarIntSize(int(v))
	case uint32:
		return getVarIntSize(int(v))
	case uint64:
		return getVarIntSize(int(v))
	case string:
		valueSize := len(v)
		return getVarIntSize(valueSize) + valueSize
	case encodable:
		cw := counterWriter{}
		v.EncodeBinary(&cw)
		if cw.err != nil {
			panic(fmt.Sprintf("error serializing %s: %s", reflect.TypeOf(value), cw.err))
		}
		return cw.counter
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		valueLength := v.Len()
		valueSize := 0

		if valueLength != 0 {
			switch reflect.ValueOf(value).Index(0).Interface().(type) {
			case encodable:
				var cw counterWriter
				cw.WriteArray(value)
				if cw.err != nil {
					panic(fmt.Errorf("invalid: %w", cw.err))
				}
				return cw.counter
			case uint8, int8:
				valueSize = valueLength
			case uint16, int16:
				valueSize = valueLength * 2
			case uint32, int32:
				valueSize = valueLength * 4
			case uint64, int64:
				valueSize = valueLength * 8
			}
		}

		return getVarIntSize(valueLength) + valueSize
	default:
		panic(fmt.Sprintf("unable to calculate GetVarSize, %s", reflect.TypeOf(value)))
	}
}
