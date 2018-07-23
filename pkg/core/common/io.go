package util

import (
	"encoding/binary"
	"io"
)

// Variable length integer, can be encoded to save space according to the value typed.
// len 1 uint8
// len 3 0xfd + uint16
// len 5 0xfe = uint32
// len 9 0xff = uint64
// For more information about this:
// https://github.com/neo-project/neo/wiki/Network-Protocol

// ReadVarUint reads a variable unsigned integer and returns it as a uint64.
func ReadVarUint(r io.Reader) uint64 {
	var b uint8
	binary.Read(r, binary.LittleEndian, &b)

	if b == 0xfd {
		var v uint16
		binary.Read(r, binary.LittleEndian, &v)
		return uint64(v)
	}
	if b == 0xfe {
		var v uint32
		binary.Read(r, binary.LittleEndian, &v)
		return uint64(v)
	}
	if b == 0xff {
		var v uint64
		binary.Read(r, binary.LittleEndian, &v)
		return v
	}

	return uint64(b)
}

// ReadVarBytes reads a variable length byte array.
func ReadVarBytes(r io.Reader) ([]byte, error) {
	n := ReadVarUint(r)
	b := make([]byte, n)
	if err := binary.Read(r, binary.LittleEndian, b); err != nil {
		return nil, err
	}
	return b, nil
}

// ReadVarString reads a variable length string.
func ReadVarString(r io.Reader) (string, error) {
	b, err := ReadVarBytes(r)
	return string(b), err
}
