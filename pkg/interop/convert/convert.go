// Package convert provides functions for type conversion.
package convert

import "github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"

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

// Uint8ToBytes converts a uint8 to a fixed-length slice.
func Uint8ToBytes(n uint8) []byte {
	return []byte{n & 0xff}
}

// BytesToUint8 converts data to a uint8.
func BytesToUint8(data []byte) uint8 {
	if len(data) != 1 {
		panic("expected data with length 1")
	}
	return data[0]
}

// Uint16ToBytesBE converts a uint16 to a fixed-length slice.
func Uint16ToBytesBE(n uint16) []byte {
	var res = make([]byte, 2)
	copy(res, ToBytes(n))
	neogointernal.Opcode1NoReturn("REVERSEITEMS", res)
	return res
}

// BytesBEToUint16 converts data in big-endian format to a uint16.
func BytesBEToUint16(data []byte) uint16 {
	if len(data) != 2 {
		panic("expected data with length 2")
	}
	src := make([]byte, 2)
	copy(src, data)
	neogointernal.Opcode1NoReturn("REVERSEITEMS", src)
	src = append(src, 0) // preserve compatibility with bigint.FromBytes.
	return uint16(ToInteger(src))
}

// Uint16ToBytesLE converts a uint16 to a fixed-length slice in little-endian format.
func Uint16ToBytesLE(n uint16) []byte {
	var res = make([]byte, 2)
	copy(res, ToBytes(n))
	return res
}

// BytesLEToUint16 converts data in little-endian format to a uint16.
func BytesLEToUint16(data []byte) uint16 {
	if len(data) != 2 {
		panic("expected data with length 2")
	}
	src := make([]byte, 3) // preserve compatibility with bigint.FromBytes.
	copy(src, data)
	return uint16(ToInteger(src))
}

// Uint32ToBytesBE converts a uint32 to a fixed-length slice in big-endian format.
func Uint32ToBytesBE(n uint32) []byte {
	var res = make([]byte, 4)
	copy(res, ToBytes(n))
	neogointernal.Opcode1NoReturn("REVERSEITEMS", res)
	return res
}

// BytesBEToUint32 converts data in big-endian format to a uint32.
func BytesBEToUint32(data []byte) uint32 {
	if len(data) != 4 {
		panic("expected data with length 4")
	}
	src := make([]byte, 4)
	copy(src, data)
	neogointernal.Opcode1NoReturn("REVERSEITEMS", src)
	src = append(src, 0) // preserve compatibility with bigint.FromBytes.
	return uint32(ToInteger(src))
}

// Uint32ToBytesLE converts a uint32 to a fixed-length slice in little-endian format.
func Uint32ToBytesLE(n uint32) []byte {
	var res = make([]byte, 4)
	copy(res, ToBytes(n))
	return res
}

// BytesLEToUint32 converts data in little-endian format to a uint32.
func BytesLEToUint32(data []byte) uint32 {
	if len(data) != 4 {
		panic("expected data with length 4")
	}
	src := make([]byte, 5) // preserve compatibility with bigint.FromBytes.
	copy(src, data)
	return uint32(ToInteger(src))
}

// Uint64ToBytesBE converts a uint64 to a fixed-length slice in big-endian format.
func Uint64ToBytesBE(n uint64) []byte {
	var res = make([]byte, 8)
	copy(res, ToBytes(n))
	neogointernal.Opcode1NoReturn("REVERSEITEMS", res)
	return res
}

// BytesBEToUint64 converts data in big-endian format to a uint64.
func BytesBEToUint64(data []byte) uint64 {
	if len(data) != 8 {
		panic("expected data with length 8")
	}
	src := make([]byte, 8)
	copy(src, data)
	neogointernal.Opcode1NoReturn("REVERSEITEMS", src)
	src = append(src, 0) // preserve compatibility with bigint.FromBytes.
	return uint64(ToInteger(src))
}

// Uint64ToBytesLE converts a uint64 to a fixed-length slice in little-endian format.
func Uint64ToBytesLE(n uint64) []byte {
	var res = make([]byte, 8)
	copy(res, ToBytes(n))
	return res
}

// BytesLEToUint64 converts data in little-endian format to a uint64.
func BytesLEToUint64(data []byte) uint64 {
	if len(data) != 8 {
		panic("expected data with length 8")
	}
	src := make([]byte, 9) // preserve compatibility with bigint.FromBytes.
	copy(src, data)
	return uint64(ToInteger(src))
}
