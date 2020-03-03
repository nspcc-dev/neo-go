package core

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// UnspentCoinState hold the state of a unspent coin.
type UnspentCoinState struct {
	states []state.Coin
}

// NewUnspentCoinState returns a new unspent coin state with N confirmed states.
func NewUnspentCoinState(n int) *UnspentCoinState {
	u := &UnspentCoinState{
		states: make([]state.Coin, n),
	}
	for i := 0; i < n; i++ {
		u.states[i] = state.CoinConfirmed
	}
	return u
}

// EncodeBinary encodes UnspentCoinState to the given BinWriter.
func (s *UnspentCoinState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarUint(uint64(len(s.states)))
	for _, state := range s.states {
		bw.WriteB(byte(state))
	}
}

// DecodeBinary decodes UnspentCoinState from the given BinReader.
func (s *UnspentCoinState) DecodeBinary(br *io.BinReader) {
	lenStates := br.ReadVarUint()
	s.states = make([]state.Coin, lenStates)
	for i := 0; i < int(lenStates); i++ {
		s.states[i] = state.Coin(br.ReadB())
	}
}
