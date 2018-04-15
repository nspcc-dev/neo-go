package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
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
func (s UnspentCoins) commit(b storage.Batch) error {
	buf := new(bytes.Buffer)
	for hash, state := range s {
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
	if err := util.WriteVarUint(w, uint64(len(s.states))); err != nil {
		return err
	}
	for _, state := range s.states {
		if err := binary.Write(w, binary.LittleEndian, byte(state)); err != nil {
			return err
		}
	}
	return nil
}

// DecodBinary decodes UnspentCoinState from the given io.Reader.
func (s *UnspentCoinState) DecodeBinary(r io.Reader) error {
	lenStates := util.ReadVarUint(r)
	s.states = make([]CoinState, lenStates)
	for i := 0; i < int(lenStates); i++ {
		var state uint8
		if err := binary.Read(r, binary.LittleEndian, &state); err != nil {
			return err
		}
		s.states[i] = CoinState(state)
	}
	return nil
}
