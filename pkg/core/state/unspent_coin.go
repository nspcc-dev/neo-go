package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// UnspentCoin hold the state of a unspent coin.
type UnspentCoin struct {
	States []Coin
}

// NewUnspentCoin returns a new unspent coin state with N confirmed states.
func NewUnspentCoin(n int) *UnspentCoin {
	u := &UnspentCoin{
		States: make([]Coin, n),
	}
	for i := 0; i < n; i++ {
		u.States[i] = CoinConfirmed
	}
	return u
}

// EncodeBinary encodes UnspentCoin to the given BinWriter.
func (s *UnspentCoin) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarUint(uint64(len(s.States)))
	for _, state := range s.States {
		bw.WriteB(byte(state))
	}
}

// DecodeBinary decodes UnspentCoin from the given BinReader.
func (s *UnspentCoin) DecodeBinary(br *io.BinReader) {
	lenStates := br.ReadVarUint()
	s.States = make([]Coin, lenStates)
	for i := 0; i < int(lenStates); i++ {
		s.States[i] = Coin(br.ReadB())
	}
}
