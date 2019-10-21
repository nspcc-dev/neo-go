package core

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// UnspentCoins is mapping between transactions and their unspent
// coin state.
type UnspentCoins map[util.Uint256]*UnspentCoinState

// getAndUpdate retreives UnspentCoinState from temporary or persistent Store
// and return it. If it's not present in both stores, returns a new
// UnspentCoinState.
func (u UnspentCoins) getAndUpdate(s storage.Store, hash util.Uint256) (*UnspentCoinState, error) {
	if unspent, ok := u[hash]; ok {
		return unspent, nil
	}

	unspent, err := getUnspentCoinStateFromStore(s, hash)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		unspent = &UnspentCoinState{
			states: []CoinState{},
		}
	}

	u[hash] = unspent
	return unspent, nil
}

// getUnspentCoinStateFromStore retrieves UnspentCoinState from the given store
func getUnspentCoinStateFromStore(s storage.Store, hash util.Uint256) (*UnspentCoinState, error) {
	unspent := &UnspentCoinState{}
	key := storage.AppendPrefix(storage.STCoin, hash.BytesReverse())
	if b, err := s.Get(key); err == nil {
		r := io.NewBinReaderFromBuf(b)
		unspent.DecodeBinary(r)
		if r.Err != nil {
			return nil, fmt.Errorf("failed to decode (UnspentCoinState): %s", r.Err)
		}
	} else {
		return nil, err
	}
	return unspent, nil
}

// putUnspentCoinStateIntoStore puts given UnspentCoinState into the given store.
func putUnspentCoinStateIntoStore(store storage.Store, hash util.Uint256, ucs *UnspentCoinState) error {
	buf := io.NewBufBinWriter()
	ucs.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	key := storage.AppendPrefix(storage.STCoin, hash.BytesReverse())
	return store.Put(key, buf.Bytes())
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
func (u UnspentCoins) commit(store storage.Store) error {
	for hash, state := range u {
		if err := putUnspentCoinStateIntoStore(store, hash, state); err != nil {
			return err
		}
	}
	return nil
}

// EncodeBinary encodes UnspentCoinState to the given BinWriter.
func (s *UnspentCoinState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarUint(uint64(len(s.states)))
	for _, state := range s.states {
		bw.WriteLE(byte(state))
	}
}

// DecodeBinary decodes UnspentCoinState from the given BinReader.
func (s *UnspentCoinState) DecodeBinary(br *io.BinReader) {
	lenStates := br.ReadVarUint()
	s.states = make([]CoinState, lenStates)
	for i := 0; i < int(lenStates); i++ {
		var state uint8
		br.ReadLE(&state)
		s.states[i] = CoinState(state)
	}
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
			r := io.NewBinReaderFromBuf(b)
			unspent.DecodeBinary(r)
			if r.Err != nil {
				return false
			}

			for _, input := range inputs {
				if int(input.PrevIndex) >= len(unspent.states) || unspent.states[input.PrevIndex] == CoinStateSpent {
					return true
				}
			}
		} else {
			return true
		}

	}

	return false
}
