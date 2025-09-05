// Package convert provides functions for type conversion.
package convert

const (
	pow2in7  = uint8(1) << 7
	pow2in15 = uint16(1) << 15
	pow2in31 = uint32(1) << 31
	pow2in63 = uint64(1) << 63
)

// ToInteger converts it's argument to an Integer.
func ToInteger(v any) int {
	return v.(int)
}

// ToBytes converts it's argument to a Buffer VM type.
func ToBytes(v any) []byte {
	return v.([]byte)
}

// ToString converts it's argument to a ByteString VM type.
func ToString(v any) string {
	return v.(string)
}

// ToBool converts it's argument to a Boolean.
func ToBool(v any) bool {
	return v.(bool)
}

// Uint8ToBytesBE converts an uint8 to a fixed-length slice in big-endian format.
func Uint8ToBytesBE(n uint8) []byte {
	return []byte{n & 0xff}
}

// BytesBEToUint8 converts data in big-endian format to an uint8.
func BytesBEToUint8(data []byte) uint8 {
	if len(data) != 1 {
		panic("expected data with length 1")
	}
	return data[0]
}

// Uint8ToBytesLE converts an uint8 to a fixed-length slice in little-endian format.
func Uint8ToBytesLE(n uint8) []byte {
	return []byte{n & 0xff}
}

// BytesLEToUint8 converts data in little-endian format to an uint8.
func BytesLEToUint8(data []byte) uint8 {
	if len(data) != 1 {
		panic("expected data with length 1")
	}
	return data[0]
}

// Uint16ToBytesBE converts an uint16 to a fixed-length slice in big-endian format.
func Uint16ToBytesBE(n uint16) []byte {
	var data = make([]byte, 2)
	data[0] = byte(n>>8) & 0xff
	data[1] = byte(n) & 0xff
	return data
}

// BytesBEToUint16 converts data in big-endian format to an uint16.
func BytesBEToUint16(data []byte) uint16 {
	if len(data) != 2 {
		panic("expected data with length 2")
	}
	//nolint:govet,staticcheck
	return uint16(data[1] | data[0]<<8)
}

// Uint16ToBytesLE converts an uint16 to a fixed-length slice in little-endian format.
func Uint16ToBytesLE(n uint16) []byte {
	var data = make([]byte, 2)
	data[1] = byte(n>>8) & 0xff
	data[0] = byte(n) & 0xff
	return data
}

// BytesLEToUint16 converts data in little-endian format to an uint16.
func BytesLEToUint16(data []byte) uint16 {
	if len(data) != 2 {
		panic("expected data with length 2")
	}
	//nolint:govet,staticcheck
	return uint16(data[0] | data[1]<<8)
}

// Uint32ToBytesBE converts an uint32 to a fixed-length slice in big-endian format.
func Uint32ToBytesBE(n uint32) []byte {
	var data = make([]byte, 4)
	data[0] = byte(n>>24) & 0xff
	data[1] = byte(n>>16) & 0xff
	data[2] = byte(n>>8) & 0xff
	data[3] = byte(n) & 0xff
	return data
}

// BytesBEToUint32 converts data in big-endian format to an uint32.
func BytesBEToUint32(data []byte) uint32 {
	if len(data) != 4 {
		panic("expected data with length 4")
	}
	//nolint:govet,staticcheck
	return uint32(data[3] | data[2]<<8 | data[1]<<16 | data[0]<<24)
}

// Uint32ToBytesLE converts an uint32 to a fixed-length slice in little-endian format.
func Uint32ToBytesLE(n uint32) []byte {
	var data = make([]byte, 4)
	data[3] = byte(n>>24) & 0xff
	data[2] = byte(n>>16) & 0xff
	data[1] = byte(n>>8) & 0xff
	data[0] = byte(n) & 0xff
	return data
}

// BytesLEToUint32 converts data in little-endian format to an uint32.
func BytesLEToUint32(data []byte) uint32 {
	if len(data) != 4 {
		panic("expected data with length 4")
	}
	//nolint:govet,staticcheck
	return uint32(data[0] | data[1]<<8 | data[2]<<16 | data[3]<<24)
}

// Uint64ToBytesBE converts an uint64 to a fixed-length slice in big-endian format.
func Uint64ToBytesBE(n uint64) []byte {
	var data = make([]byte, 8)
	data[0] = byte(n>>56) & 0xff
	data[1] = byte(n>>48) & 0xff
	data[2] = byte(n>>40) & 0xff
	data[3] = byte(n>>32) & 0xff
	data[4] = byte(n>>24) & 0xff
	data[5] = byte(n>>16) & 0xff
	data[6] = byte(n>>8) & 0xff
	data[7] = byte(n) & 0xff
	return data
}

// BytesBEToUint64 converts data in big-endian format to an uint64.
func BytesBEToUint64(data []byte) uint64 {
	if len(data) != 8 {
		panic("expected data with length 8")
	}
	//nolint:govet,staticcheck
	return uint64(data[7] | data[6]<<8 | data[5]<<16 | data[4]<<24 |
		data[3]<<32 | data[2]<<40 | data[1]<<48 | data[0]<<56)
}

// Uint64ToBytesLE converts an uint64 to a fixed-length slice in little-endian format.
func Uint64ToBytesLE(n uint64) []byte {
	var data = make([]byte, 8)
	data[7] = byte(n>>56) & 0xff
	data[6] = byte(n>>48) & 0xff
	data[5] = byte(n>>40) & 0xff
	data[4] = byte(n>>32) & 0xff
	data[3] = byte(n>>24) & 0xff
	data[2] = byte(n>>16) & 0xff
	data[1] = byte(n>>8) & 0xff
	data[0] = byte(n) & 0xff
	return data
}

// BytesLEToUint64 converts data in little-endian format to an uint64.
func BytesLEToUint64(data []byte) uint64 {
	if len(data) != 8 {
		panic("expected data with length 8")
	}
	//nolint:govet,staticcheck
	return uint64(data[0] | data[1]<<8 | data[2]<<16 | data[3]<<24 |
		data[4]<<32 | data[5]<<40 | data[6]<<48 | data[7]<<56)
}

// Int8ToBytesBE converts an int8 to a fixed-length slice in big-endian format.
func Int8ToBytesBE(n int8) []byte {
	return Uint8ToBytesBE(uint8(n) + pow2in7)
}

// BytesBEToInt8 converts data in big-endian format to an int8.
func BytesBEToInt8(data []byte) int8 {
	return int8(BytesBEToUint8(data) - pow2in7)
}

// Int8ToBytesLE converts an int8 to a fixed-length slice in little-endian format.
func Int8ToBytesLE(n int8) []byte {
	return Uint8ToBytesLE(uint8(n) + pow2in7)
}

// BytesLEToInt8 converts data in little-endian format to an int8.
func BytesLEToInt8(data []byte) int8 {
	return int8(BytesLEToUint8(data) - pow2in7)
}

// Int16ToBytesBE converts an int16 to a fixed-length slice in big-endian format.
func Int16ToBytesBE(n int16) []byte {
	return Uint16ToBytesBE(uint16(n) + pow2in15)
}

// BytesBEToInt16 converts data in big-endian format to an int16.
func BytesBEToInt16(data []byte) int16 {
	return int16(BytesBEToUint16(data) - pow2in15)
}

// Int16ToBytesLE converts an int16 to a fixed-length slice in little-endian format.
func Int16ToBytesLE(n int16) []byte {
	return Uint16ToBytesLE(uint16(n) + pow2in15)
}

// BytesLEToInt16 converts data in little-endian format to an int16.
func BytesLEToInt16(data []byte) int16 {
	return int16(BytesLEToUint16(data) - pow2in15)
}

// Int32ToBytesBE converts an int32 to a fixed-length slice in big-endian format.
func Int32ToBytesBE(n int32) []byte {
	return Uint32ToBytesBE(uint32(n) + pow2in31)
}

// BytesBEToInt32 converts data in big-endian format to an int32.
func BytesBEToInt32(data []byte) int32 {
	return int32(BytesBEToUint32(data) - pow2in31)
}

// Int32ToBytesLE converts an int32 to a fixed-length slice in little-endian format.
func Int32ToBytesLE(n int32) []byte {
	return Uint32ToBytesLE(uint32(n) + pow2in31)
}

// BytesLEToInt32 converts data in little-endian format to an int32.
func BytesLEToInt32(data []byte) int32 {
	return int32(BytesLEToUint32(data) - pow2in31)
}

// Int64ToBytesBE converts an int64 to a fixed-length slice in big-endian format.
func Int64ToBytesBE(n int64) []byte {
	return Uint64ToBytesBE(uint64(n) + pow2in63)
}

// BytesBEToInt64 converts data in big-endian format to an int64.
func BytesBEToInt64(data []byte) int64 {
	return int64(BytesBEToUint64(data) - pow2in63)
}

// Int64ToBytesLE converts an int64 to a fixed-length slice in little-endian format.
func Int64ToBytesLE(n int64) []byte {
	return Uint64ToBytesLE(uint64(n) + pow2in63)
}

// BytesLEToInt64 converts data in little-endian format to an int64.
func BytesLEToInt64(data []byte) int64 {
	return int64(BytesLEToUint64(data) - pow2in63)
}
