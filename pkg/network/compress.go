package network

import (
	"encoding/binary"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/pierrec/lz4"
)

// compress compresses bytes using lz4.
func compress(source []byte) ([]byte, error) {
	dest := make([]byte, 4+lz4.CompressBlockBound(len(source)))
	size, err := lz4.CompressBlock(source, dest[4:], nil)
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(dest[:4], uint32(len(source)))
	return dest[:size+4], nil
}

// decompress decompresses bytes using lz4.
func decompress(source []byte) ([]byte, error) {
	if len(source) < 4 {
		return nil, errors.New("invalid compressed payload")
	}
	length := binary.LittleEndian.Uint32(source[:4])
	if length > payload.MaxSize {
		return nil, errors.New("invalid uncompressed payload length")
	}
	dest := make([]byte, length)
	size, err := lz4.UncompressBlock(source[4:], dest)
	if err != nil {
		return nil, err
	}
	if uint32(size) != length {
		return nil, errors.New("decompressed payload size doesn't match header")
	}
	return dest, nil
}
