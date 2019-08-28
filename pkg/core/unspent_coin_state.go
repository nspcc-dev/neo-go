package core

import (
	"bytes"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// UnspentCoins is mapping between transactions and their unspent
// coin state.
type UnspentCoins map[util.Uint256]*UnspentCoinState

func (u UnspentCoins) getAndUpdate(s storage.Store, hash util.Uint256) (*UnspentCoinState, error) {
	if unspent, ok := u[hash]; ok {
		return unspent, nil
	}

	unspent := &UnspentCoinState{}
	key := storage.AppendPrefix(storage.STCoin, hash.BytesReverse())
	if b, err := s.Get(key); err == nil {
		if err := unspent.DecodeBinary(bytes.NewReader(b)); err != nil {
			return nil, fmt.Errorf("failed to decode (UnspentCoinState): %s", err)
		}
	} else {
		unspent = &UnspentCoinState{
			states: []CoinState{},
		}
	}

	u[hash] = unspent
	return unspent, nil
}

// UnspentCoinState hold the state of a unspent coin.
type UnspentCoinState struct {
	states []CoinState
}

// NewUnspentCoinState returns a new unspent coin state with N confirmed states.
func NewUnspentCoinState(n int) *UnspentCoinState {
	u := &UnspentCoinState{
		states: make([]CoinState, n),
	}
	for i := 0; i < n; i++ {
		u.states[i] = CoinStateConfirmed
	}
	return u
}

// commit writes all unspent coin states to the given Batch.
func (u UnspentCoins) commit(b storage.Batch) error {
	buf := new(bytes.Buffer)
	for hash, state := range u {
		if err := state.EncodeBinary(buf); err != nil {
			return err
		}
		key := storage.AppendPrefix(storage.STCoin, hash.BytesReverse())
		b.Put(key, buf.Bytes())
		buf.Reset()
	}
	return nil
}

// EncodeBinary encodes UnspentCoinState to the given io.Writer.
func (s *UnspentCoinState) EncodeBinary(w io.Writer) error {
	bw := util.BinWriter{W: w}
	bw.WriteVarUint(uint64(len(s.states)))
	for _, state := range s.states {
		bw.WriteLE(byte(state))
	}
	return bw.Err
}

// DecodeBinary decodes UnspentCoinState from the given io.Reader.
func (s *UnspentCoinState) DecodeBinary(r io.Reader) error {
	br := util.BinReader{R: r}
	lenStates := br.ReadVarUint()
	s.states = make([]CoinState, lenStates)
	for i := 0; i < int(lenStates); i++ {
		var state uint8
		br.ReadLE(&state)
		s.states[i] = CoinState(state)
	}
	return br.Err
}

// IsDoubleSpend verifies that the input transactions are not double spent.
func IsDoubleSpend(s storage.Store, tx *transaction.Transaction) bool {
	if len(tx.Inputs) == 0 {
		return false
	}

	for prevHash, inputs := range tx.GroupInputsByPrevHash() {
		unspent := &UnspentCoinState{}
		key := storage.AppendPrefix(storage.STCoin, prevHash.BytesReverse())
		if b, err := s.Get(key); err == nil {
			if err := unspent.DecodeBinary(bytes.NewReader(b)); err != nil {
				return false
			}
			if unspent == nil {
				return true
			}

			for _, input := range inputs {
				if int(input.PrevIndex) >= len(unspent.states) || unspent.states[input.PrevIndex] == CoinStateSpent {
					return true
				}
			}
		}

	}

	return false
}
