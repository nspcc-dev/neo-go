// Package convert provides functions for type conversion.
package convert

const (
	// uintSizeInBytes is the size of a uint in bytes.
	uintSizeInBytes = (32 << (^uint(0) >> 63)) / 8
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

// FromUintToFixedBEBytes converts an uint64 to a slice of a specified length
// in big-endian format.
func FromUintToFixedBEBytes(m uint64, width uint64) []byte {
	var n = m
	if width == 0 {
		return []byte{}
	}
	var res = make([]byte, width)
	for i := int64(width - 1); i >= 0; i-- {
		res[i] = byte(n & 0xff)
		n = n >> 8
	}
	return res
}

// FromUint64ToBEBytes converts an uint64 to a fixed-length slice in big-endian format.
func FromUint64ToBEBytes(n uint64) []byte {
	return FromUintToFixedBEBytes(n, 8)
}

// FromUint32ToBEBytes converts an uint32 to a fixed-length slice in big-endian format.
func FromUint32ToBEBytes(n uint32) []byte {
	return FromUintToFixedBEBytes(uint64(n), 4)
}

// FromUint16ToBEBytes converts an uint16 to a fixed-length slice in big-endian format.
func FromUint16ToBEBytes(n uint16) []byte {
	return FromUintToFixedBEBytes(uint64(n), 2)
}

// FromUint8ToBEBytes converts an uint8 to a fixed-length slice in big-endian format.
func FromUint8ToBEBytes(n uint8) []byte {
	return FromUintToFixedBEBytes(uint64(n), 1)
}

// FromUintToBEBytes converts an uint to a fixed-length slice in big-endian format.
func FromUintToBEBytes(n uint) []byte {
	return FromUintToFixedBEBytes(uint64(n), uintSizeInBytes)
}

// FromIntToFixedBEBytes converts an int64 to a slice of a specified length
// in big-endian format.
func FromIntToFixedBEBytes(m int64, width uint64) []byte {
	var n = m
	if width == 0 {
		return []byte{}
	}
	n = n + (1 << (width*8 - 1))
	return FromUintToFixedBEBytes(uint64(n), width)
}

// FromInt64ToBEBytes converts an int64 to a fixed-length slice in big-endian format.
func FromInt64ToBEBytes(n int64) []byte {
	return FromIntToFixedBEBytes(n, 8)
}

// FromInt32ToBEBytes converts an int32 to a fixed-length slice in big-endian format.
func FromInt32ToBEBytes(n int32) []byte {
	return FromIntToFixedBEBytes(int64(n), 4)
}

// FromInt16ToBEBytes converts an int16 to a fixed-length slice in big-endian format.
func FromInt16ToBEBytes(n int16) []byte {
	return FromIntToFixedBEBytes(int64(n), 2)
}

// FromInt8ToBEBytes converts an int8 to a fixed-length slice in big-endian format.
func FromInt8ToBEBytes(n int8) []byte {
	return FromIntToFixedBEBytes(int64(n), 1)
}

// FromIntToBEBytes converts an int to a fixed-length slice in big-endian format.
func FromIntToBEBytes(n int) []byte {
	return FromIntToFixedBEBytes(int64(n), uintSizeInBytes)
}

// FromBEBytesToUint64 converts data in big-endian format to an uint64.
func FromBEBytesToUint64(data []byte) uint64 {
	var n uint64
	//nolint:all // integer range not supported
	for i := 0; i < len(data); i++ {
		n = n << 8
		n = n | uint64(data[i])
	}
	return n
}

// FromBEBytesToUint32 converts data in big-endian format to an uint32.
func FromBEBytesToUint32(data []byte) uint32 {
	return uint32(FromBEBytesToUint64(data))
}

// FromBEBytesToUint16 converts data in big-endian format to an uint16.
func FromBEBytesToUint16(data []byte) uint16 {
	return uint16(FromBEBytesToUint64(data))
}

// FromBEBytesToUint8 converts data in big-endian format to an uint8.
func FromBEBytesToUint8(data []byte) uint8 {
	return uint8(FromBEBytesToUint64(data))
}

// FromBEBytesToUint converts data in big-endian format to an uint.
func FromBEBytesToUint(data []byte) uint {
	return uint(FromBEBytesToUint64(data))
}

// FromBEBytesToInt64 converts data in big-endian format to an int64.
func FromBEBytesToInt64(buf []byte) int64 {
	var (
		n    = int64(FromBEBytesToUint64(buf))
		bits = 8 * len(buf)
	)
	return n - (1 << (bits - 1))
}

// FromBEBytesToInt32 converts data in big-endian format to an int32.
func FromBEBytesToInt32(data []byte) int32 {
	return int32(FromBEBytesToInt64(data))
}

// FromBEBytesToInt16 converts data in big-endian format to an int16.
func FromBEBytesToInt16(data []byte) int16 {
	return int16(FromBEBytesToInt64(data))
}

// FromBEBytesToInt8 converts data in big-endian format to an int8.
func FromBEBytesToInt8(data []byte) int8 {
	return int8(FromBEBytesToInt64(data))
}

// FromBEBytesToInt converts data in big-endian format to an int.
func FromBEBytesToInt(data []byte) int {
	return int(FromBEBytesToInt64(data))
}
