package util

import (
	"bytes"
	"crypto/sha256"

	"io"
	"io/ioutil"
)

// Functions
func BufferLength(buf *bytes.Buffer) uint32 {

	return uint32(buf.Len())
}

func SumSHA256(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)
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

func ReaderToBuffer(r io.Reader) (buf *bytes.Buffer, err error) {
	byt, err := ioutil.ReadAll(r)

	if err != nil {

		return
	}

	buf = bytes.NewBuffer(byt)

	return
}
