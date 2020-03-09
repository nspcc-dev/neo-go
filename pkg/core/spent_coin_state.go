package core

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// SpentCoinState represents the state of a spent coin.
type SpentCoinState struct {
	txHeight uint32

	// A mapping between the index of the prevIndex and block height.
	items map[uint16]uint32
}

// spentCoin represents the state of a single spent coin output.
type spentCoin struct {
	Output      *transaction.Output
	StartHeight uint32
	EndHeight   uint32
}

// NewSpentCoinState returns a new SpentCoinState object.
func NewSpentCoinState(height uint32) *SpentCoinState {
	return &SpentCoinState{
		txHeight: height,
		items:    make(map[uint16]uint32),
	}
}

// DecodeBinary implements Serializable interface.
func (s *SpentCoinState) DecodeBinary(br *io.BinReader) {
	s.txHeight = br.ReadU32LE()

	s.items = make(map[uint16]uint32)
	lenItems := br.ReadVarUint()
	for i := 0; i < int(lenItems); i++ {
		var (
			key   uint16
			value uint32
		)
		key = br.ReadU16LE()
		value = br.ReadU32LE()
		s.items[key] = value
	}
}

// EncodeBinary implements Serializable interface.
func (s *SpentCoinState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(s.txHeight)
	bw.WriteVarUint(uint64(len(s.items)))
	for k, v := range s.items {
		bw.WriteU16LE(k)
		bw.WriteU32LE(v)
	}
}
