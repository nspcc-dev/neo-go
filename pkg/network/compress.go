package network

import (
	"github.com/pierrec/lz4"
)

// compress compresses bytes using lz4.
func compress(source []byte) ([]byte, error) {
	dest := make([]byte, lz4.CompressBlockBound(len(source)))
	size, err := lz4.CompressBlock(source, dest, nil)
	if err != nil {
		return nil, err
	}
	return dest[:size], nil
}

// decompress decompresses bytes using lz4.
func decompress(source []byte) ([]byte, error) {
	maxSize := len(source) * 255
	if maxSize > PayloadMaxSize {
		maxSize = PayloadMaxSize
	}
	dest := make([]byte, maxSize)
	size, err := lz4.UncompressBlock(source, dest)
	if err != nil {
		return nil, err
	}
	return dest[:size], nil
}
