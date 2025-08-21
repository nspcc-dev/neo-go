// Package convert provides functions for type conversion.
package convert

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

// FromUint64ToBEBytes converts an uint64 to a slice in big-endian format.
func FromUint64ToBEBytes(n uint64) []byte {
	if n == 0 {
		return []byte{}
	}
	var data []byte
	for n > 0 {
		data = append(data, byte(n&0xFF))
		n = n >> 8
	}
	if data[len(data)-1]&0x80 != 0 {
		data = append(data, 0)
	}
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
	return data
}

// FromUint32ToBEBytes converts an uint32 to a slice in big-endian format.
func FromUint32ToBEBytes(n uint32) []byte {
	return FromUint64ToBEBytes(uint64(n))
}

// FromUint16ToBEBytes converts an uint16 to a slice in big-endian format.
func FromUint16ToBEBytes(n uint16) []byte {
	return FromUint64ToBEBytes(uint64(n))
}

// FromUint8ToBEBytes converts an uint8 to a slice in big-endian format.
func FromUint8ToBEBytes(n uint8) []byte {
	return FromUint64ToBEBytes(uint64(n))
}

// FromUintToBEBytes converts an uint to a slice in big-endian format.
func FromUintToBEBytes(n uint) []byte {
	return FromUint64ToBEBytes(uint64(n))
}

// FromInt64ToBEBytes converts an int64 to a slice in big-endian format.
func FromInt64ToBEBytes(m int64) []byte {
	var n = m
	if n == 0 {
		return []byte{}
	}
	isNeg := n < 0
	if isNeg {
		n *= -1
		n -= 1
		if n == 0 {
			return []byte{0xFF}
		}
	}
	var data []byte
	for n > 0 {
		data = append(data, byte(n&0xFF))
		n = n >> 8
	}
	if data[len(data)-1]&0x80 != 0 {
		data = append(data, 0)
	}
	if isNeg {
		for i := 0; i < len(data); i++ {
			data[i] = ^data[i] & 0xFF
		}
	}
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
	return data
}

func MyFunc(n int) int {
	var m = n
	return m
}

// FromInt32ToBEBytes converts an int32 to a slice in big-endian format.
func FromInt32ToBEBytes(n int32) []byte {
	return FromInt64ToBEBytes(int64(n))
}

// FromInt16ToBEBytes converts an int16 to a slice in big-endian format.
func FromInt16ToBEBytes(n int16) []byte {
	return FromInt64ToBEBytes(int64(n))
}

// FromInt8ToBEBytes converts an int8 to a slice in big-endian format.
func FromInt8ToBEBytes(n int8) []byte {
	return FromInt64ToBEBytes(int64(n))
}

// FromIntToBEBytes converts an int to a slice in big-endian format.
func FromIntToBEBytes(n int) []byte {
	return FromInt64ToBEBytes(int64(n))
}

// FromBEBytesToUint64 converts data in big-endian format to an uint64.
func FromBEBytesToUint64(data []byte) uint64 {
	var n uint64
	for _, v := range data {
		n = n << 8
		n = n | uint64(v)
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
	if len(buf) == 0 {
		return 0
	}
	var data []byte
	for i := 0; i < len(buf); i++ {
		data = append(data, buf[i])
	}
	var isNeg = data[0]&0x80 != 0
	if isNeg {
		for i := range data {
			data[i] = ^data[i] & 0xFF
		}
	}
	data = data[getOffset(data):]
	if len(data) == 0 {
		if isNeg {
			return -1
		}
		return 0
	}
	var n int64
	for _, v := range data {
		n = n << 8
		n = n | int64(v)
	}
	if isNeg {
		n += 1
		n *= -1
	}
	return n
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

// getOffset returns the index of the first non-zero byte in the slice.
func getOffset(data []byte) int {
	for offset := range data {
		if data[offset] != 0 {
			return offset
		}
	}
	return len(data)
}
