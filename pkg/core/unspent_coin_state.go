package core

import (
	"github.com/CityOfZion/neo-go/pkg/core/entities"
	"github.com/CityOfZion/neo-go/pkg/io"
)

// UnspentCoinState hold the state of a unspent coin.
type UnspentCoinState struct {
	states []entities.CoinState
}

// NewUnspentCoinState returns a new unspent coin state with N confirmed states.
func NewUnspentCoinState(n int) *UnspentCoinState {
	u := &UnspentCoinState{
		states: make([]entities.CoinState, n),
	}
	for i := 0; i < n; i++ {
		u.states[i] = entities.CoinStateConfirmed
	}
	return u
}

// EncodeBinary encodes UnspentCoinState to the given BinWriter.
func (s *UnspentCoinState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarUint(uint64(len(s.states)))
	for _, state := range s.states {
		bw.WriteBytes([]byte{byte(state)})
	}
}

// DecodeBinary decodes UnspentCoinState from the given BinReader.
func (s *UnspentCoinState) DecodeBinary(br *io.BinReader) {
	lenStates := br.ReadVarUint()
	s.states = make([]entities.CoinState, lenStates)
	for i := 0; i < int(lenStates); i++ {
		var state uint8
		br.ReadLE(&state)
		s.states[i] = entities.CoinState(state)
	}
}
