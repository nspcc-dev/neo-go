package state

import "github.com/nspcc-dev/neo-go/pkg/io"

// SpentCoin represents the state of a spent coin.
type SpentCoin struct {
	TxHeight uint32

	// A mapping between the index of the prevIndex and block height.
	Items map[uint16]uint32
}

// NewSpentCoin returns a new SpentCoin object.
func NewSpentCoin(height uint32) *SpentCoin {
	return &SpentCoin{
		TxHeight: height,
		Items:    make(map[uint16]uint32),
	}
}

// DecodeBinary implements Serializable interface.
func (s *SpentCoin) DecodeBinary(br *io.BinReader) {
	s.TxHeight = br.ReadU32LE()

	s.Items = make(map[uint16]uint32)
	lenItems := br.ReadVarUint()
	for i := 0; i < int(lenItems); i++ {
		var (
			key   uint16
			value uint32
		)
		key = br.ReadU16LE()
		value = br.ReadU32LE()
		s.Items[key] = value
	}
}

// EncodeBinary implements Serializable interface.
func (s *SpentCoin) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(s.TxHeight)
	bw.WriteVarUint(uint64(len(s.Items)))
	for k, v := range s.Items {
		bw.WriteU16LE(k)
		bw.WriteU32LE(v)
	}
}
