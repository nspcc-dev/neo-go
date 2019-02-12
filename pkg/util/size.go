package util

import (
	"fmt"
	"reflect"

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
// like Uint160, Uint256 are not supported. @TODO: make sure to have full unit tests coverage.
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

		// workaround for case v is a slice / Array of io.Serializable
		typeArray := reflect.TypeOf(value).Elem()
		SerializableType := reflect.TypeOf((*io.Serializable)(nil)).Elem()
		if typeArray.Implements(SerializableType) {
			for i := 0; i < valueLength; i++ {
				elem := v.Index(i).Interface().(io.Serializable)
				valueSize += elem.Size()
			}
		}

		switch value.(type) {
		case []uint8, []int8,
			Uint160, Uint256,
			[20]uint8, [32]uint8,
			[20]int8, [32]int8:
			fmt.Println("t []uint8")
			valueSize = valueLength
		case []uint16, []int16,
			[10]uint16, [10]int16:
			fmt.Println("t []uint16")
			valueSize = valueLength * 2
		case []uint32, []int32,
			[30]uint32, [30]int32:
			fmt.Println("t []uint32")
			valueSize = valueLength * 4
		case []uint64, []int64,
			[30]uint64, [30]int64:
			fmt.Println("t", "[]uint64")
			valueSize = valueLength * 8
		}

		return GetVarIntSize(valueLength) + valueSize
	default:
		panic(fmt.Sprintf("unable to calculate GetVarSize, %s", reflect.TypeOf(value)))
	}
}
