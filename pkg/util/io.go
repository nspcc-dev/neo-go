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
	switch b {
	case 0xfd:
		var v uint16
		binary.Read(r, binary.LittleEndian, &v)
		return uint64(v)
	case 0xfe:
		var v uint32
		binary.Read(r, binary.LittleEndian, &v)
		return uint64(v)
	case 0xff:
		var v uint64
		binary.Read(r, binary.LittleEndian, &v)
		return v
	default:
		return uint64(b)
	}
}

// WriteVarUint writes a variable unsigned integer.
func WriteVarUint(w io.Writer, val uint64) error {
	if val < 0xfd {
		return binary.Write(w, binary.LittleEndian, uint8(val))
	}
	if val < 0xFFFF {
		if err := binary.Write(w, binary.LittleEndian, byte(0xfd)); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, uint16(val))
	}
	if val < 0xFFFFFFFF {
		if err := binary.Write(w, binary.LittleEndian, byte(0xfe)); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, uint32(val))
	}

	if err := binary.Write(w, binary.LittleEndian, byte(0xff)); err != nil {
		return err
	}

	return binary.Write(w, binary.LittleEndian, val)
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

// WriteVarString writes a variable length string.
func WriteVarString(w io.Writer, s string) error {
	return WriteVarBytes(w, []byte(s))
}

// WriteVarBytes writes a variable length byte array.
func WriteVarBytes(w io.Writer, b []byte) error {
	if err := WriteVarUint(w, uint64(len(b))); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, b)
}
