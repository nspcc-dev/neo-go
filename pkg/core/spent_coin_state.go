package core

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// SpentCoinState represents the state of a spent coin.
type SpentCoinState struct {
	txHash   util.Uint256
	txHeight uint32

	// A mapping between the index of the prevIndex and block height.
	items map[uint16]uint32
}

// NewSpentCoinState returns a new SpentCoinState object.
func NewSpentCoinState(hash util.Uint256, height uint32) *SpentCoinState {
	return &SpentCoinState{
		txHash:   hash,
		txHeight: height,
		items:    make(map[uint16]uint32),
	}
}

// DecodeBinary implements Serializable interface.
func (s *SpentCoinState) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(s.txHash[:])
	br.ReadLE(&s.txHeight)

	s.items = make(map[uint16]uint32)
	lenItems := br.ReadVarUint()
	for i := 0; i < int(lenItems); i++ {
		var (
			key   uint16
			value uint32
		)
		br.ReadLE(&key)
		br.ReadLE(&value)
		s.items[key] = value
	}
}

// EncodeBinary implements Serializable interface.
func (s *SpentCoinState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteBytes(s.txHash[:])
	bw.WriteLE(s.txHeight)
	bw.WriteVarUint(uint64(len(s.items)))
	for k, v := range s.items {
		bw.WriteLE(k)
		bw.WriteLE(v)
	}
}
