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
}

// Write implements the io.Writer interface.
func (cw *counterWriter) Write(p []byte) (int, error) {
	n := len(p)
	cw.counter += n
	return n, nil
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
// them with variable-length encoding that is used in NEO), strings, pointers to Serializable structures,
// slices and arrays of ints/uints or Serializable structures. It's similar to GetVarSize<T>(this T[] value)
// used in C#, but differs in that it also supports things like Uint160 or Uint256.
func GetVarSize(value interface{}) int {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		valueSize := len([]byte(v.String()))
		return getVarIntSize(valueSize) + valueSize
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return getVarIntSize(int(v.Int()))
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return getVarIntSize(int(v.Uint()))
	case reflect.Ptr:
		vser, ok := v.Interface().(Serializable)
		if !ok {
			panic("unable to calculate GetVarSize for a non-Serializable pointer")
		}
		cw := counterWriter{}
		w := NewBinWriterFromIO(&cw)
		vser.EncodeBinary(w)
		if w.Err != nil {
			panic(fmt.Sprintf("error serializing %s: %s", reflect.TypeOf(value), w.Err.Error()))
		}
		return cw.counter
	case reflect.Slice, reflect.Array:
		valueLength := v.Len()
		valueSize := 0

		if valueLength != 0 {
			switch reflect.ValueOf(value).Index(0).Interface().(type) {
			case Serializable:
				for i := 0; i < valueLength; i++ {
					valueSize += GetVarSize(v.Index(i).Interface())
				}
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
