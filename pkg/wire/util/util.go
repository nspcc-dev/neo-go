package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

// Uint160 array
const uint160Size = 20

type Uint160 [uint160Size]uint8

// Type for CMD

// Functions
func CalculatePayloadLength(buf *bytes.Buffer) uint32 {

	return uint32(buf.Len())
}
func CalculateCheckSum(buf *bytes.Buffer) uint32 {

	checksum := SumSHA256(SumSHA256(buf.Bytes()))
	return binary.LittleEndian.Uint32(checksum[:4])
}

func SumSHA256(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)
}

func CompareChecksum(have uint32, b []byte) bool {
	sum := SumSHA256(SumSHA256(b))[:4]
	want := binary.LittleEndian.Uint32(sum)
	return have == want
}

func CalculateHash(f func(bw *BinWriter)) (Uint256, error) {
	buf := new(bytes.Buffer)
	bw := &BinWriter{W: buf}

	f(bw)

	var hash Uint256
	hash = sha256.Sum256(buf.Bytes())
	hash = sha256.Sum256(hash.Bytes())
	return hash, bw.Err

}
