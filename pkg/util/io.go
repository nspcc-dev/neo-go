package util

import (
	"bytes"
	"encoding/binary"
	"errors"
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

// WriteVarUint writes a variable unsigned integer.
func WriteVarUint(w io.Writer, val uint64) error {
	if val < 0 {
		return errors.New("value out of range")
	}
	if val < 0xfd {
		binary.Write(w, binary.LittleEndian, uint8(val))
		return nil
	}
	if val < 0xFFFF {
		binary.Write(w, binary.LittleEndian, byte(0xfd))
		binary.Write(w, binary.LittleEndian, uint16(val))
		return nil
	}
	if val < 0xFFFFFFFF {
		binary.Write(w, binary.LittleEndian, byte(0xfe))
		binary.Write(w, binary.LittleEndian, uint32(val))
		return nil
	}

	binary.Write(w, binary.LittleEndian, byte(0xff))
	binary.Write(w, binary.LittleEndian, val)

	return nil
}

// Read2000Uint256Hashes attempt to read 2000 Uint256 hashes from
// the given byte array.
func Read2000Uint256Hashes(b []byte) ([]Uint256, error) {
	r := bytes.NewReader(b)
	lenHashes := ReadVarUint(r)
	hashes := make([]Uint256, lenHashes)
	if err := binary.Read(r, binary.LittleEndian, hashes); err != nil {
		return nil, err
	}
	return hashes, nil
}
