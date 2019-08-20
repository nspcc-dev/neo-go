package util

import (
	"bytes"
	"crypto/sha256"

	"io"
	"io/ioutil"
)

// Convenience function

// BufferLength returns the length of a buffer as uint32
func BufferLength(buf *bytes.Buffer) uint32 {

	return uint32(buf.Len())
}

// SumSHA256 returns the sha256 sum of the data
func SumSHA256(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)
}

// CalculateHash takes a function with a binary writer and returns
// the double hash of the io.Writer
func CalculateHash(f func(bw *BinWriter)) (Uint256, error) {
	buf := new(bytes.Buffer)
	bw := &BinWriter{W: buf}

	f(bw)

	var hash Uint256
	hash = sha256.Sum256(buf.Bytes())
	hash = sha256.Sum256(hash.Bytes())
	return hash, bw.Err

}

//ReaderToBuffer converts a io.Reader into a bytes.Buffer
func ReaderToBuffer(r io.Reader) (*bytes.Buffer, error) {
	byt, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(byt)
	return buf, nil
}
