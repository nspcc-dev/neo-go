package network

import (
	"bytes"
	"io"

	"github.com/pierrec/lz4"
)

// compress compresses bytes using lz4.
func compress(source []byte) ([]byte, error) {
	dest := new(bytes.Buffer)
	w := lz4.NewWriter(dest)
	_, err := io.Copy(w, bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	if w.Close() != nil {
		return nil, err
	}
	return dest.Bytes(), nil
}

// decompress decompresses bytes using lz4.
func decompress(source []byte) ([]byte, error) {
	dest := new(bytes.Buffer)
	r := lz4.NewReader(bytes.NewReader(source))
	_, err := io.Copy(dest, r)
	if err != nil {
		return nil, err
	}
	return dest.Bytes(), nil
}
