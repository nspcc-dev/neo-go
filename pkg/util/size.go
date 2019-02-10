package util

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/CityOfZion/neo-go/pkg/io"
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
		size = unsafe.Sizeof(bit8)
	} else if value <= 0xFFFF {
		size = unsafe.Sizeof(bit8) + unsafe.Sizeof(ui16)
	} else {
		size = unsafe.Sizeof(bit8) + unsafe.Sizeof(ui32)
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
// like Uint160, Uint256 are not supported. @TODO: make sure to have full unit tests coverage.
func GetVarSize(value interface{}) int {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return GetVarStringSize(v.String())
	case reflect.Int:
		return GetVarIntSize(int(v.Int()))
	case reflect.Slice, reflect.Array:
		valueLength := v.Len()
		valueSize := 0

		// case v is a slice / Array of io.Serializable
		t := reflect.TypeOf(value).Elem()
		SerializableType := reflect.TypeOf((*io.Serializable)(nil)).Elem()
		if t.Implements(SerializableType) {
			for i := 0; i < valueLength; i++ {
				elem := v.Index(i).Interface().(io.Serializable)
				valueSize += elem.Size()
			}
		} else if t == reflect.TypeOf(bit8) || t == reflect.TypeOf(ui8) || t == reflect.TypeOf(i8) {
			valueSize = valueLength
		} else if t == reflect.TypeOf(ui16) || t == reflect.TypeOf(i16) {
			valueSize = valueLength * 2
		} else if t == reflect.TypeOf(ui32) || t == reflect.TypeOf(i32) {
			valueSize = valueLength * 4
		} else if t == reflect.TypeOf(ui64) || t == reflect.TypeOf(i64) {
			valueSize = valueLength * 8
		}

		return GetVarIntSize(valueLength) + valueSize
	default:
		panic(fmt.Sprintf("unable to calculate GetVarSize, %s", reflect.TypeOf(value)))
	}
}
