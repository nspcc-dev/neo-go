package io

import (
	"fmt"
	"reflect"
)

var (
	bit8 byte
	ui8  uint8
	ui16 uint16
	ui32 uint32
	ui64 uint64
	i8   int8
	i16  int16
	i32  int32
	i64  int64
)

// GetVarIntSize returns the size in number of bytes of a variable integer
// (reference: GetVarSize(int value),  https://github.com/neo-project/neo/blob/master/neo/IO/Helper.cs)
func GetVarIntSize(value int) int {
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

// GetVarStringSize returns the size of a variable string
// (reference: GetVarSize(this string value),  https://github.com/neo-project/neo/blob/master/neo/IO/Helper.cs)
func GetVarStringSize(value string) int {
	valueSize := len([]byte(value))
	return GetVarIntSize(valueSize) + valueSize
}

// GetVarSize return the size om bytes of a variable. This implementation is not exactly like the C#
// (reference: GetVarSize<T>(this T[] value), https://github.com/neo-project/neo/blob/master/neo/IO/Helper.cs#L53) as in the C# variable
// like Uint160, Uint256 are not supported.
func GetVarSize(value interface{}) int {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return GetVarStringSize(v.String())
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return GetVarIntSize(int(v.Int()))
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return GetVarIntSize(int(v.Uint()))
	case reflect.Slice, reflect.Array:
		valueLength := v.Len()
		valueSize := 0

		if valueLength != 0 {
			switch reflect.ValueOf(value).Index(0).Interface().(type) {
			case Serializable:
				for i := 0; i < valueLength; i++ {
					elem := v.Index(i).Interface().(Serializable)
					valueSize += elem.Size()
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

		return GetVarIntSize(valueLength) + valueSize
	default:
		panic(fmt.Sprintf("unable to calculate GetVarSize, %s", reflect.TypeOf(value)))
	}
}
