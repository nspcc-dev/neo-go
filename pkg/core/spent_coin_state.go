package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// SpentCoins is mapping between transactions and their spent
// coin state.
type SpentCoins map[util.Uint256]*SpentCoinState

func (s SpentCoins) getAndUpdate(store storage.Store, hash util.Uint256) (*SpentCoinState, error) {
	if spent, ok := s[hash]; ok {
		return spent, nil
	}

	spent := &SpentCoinState{}
	key := storage.AppendPrefix(storage.STSpentCoin, hash.BytesReverse())
	if b, err := store.Get(key); err == nil {
		if err := spent.DecodeBinary(bytes.NewReader(b)); err != nil {
			return nil, fmt.Errorf("failed to decode (UnspentCoinState): %s", err)
		}
	} else {
		spent = &SpentCoinState{
			items: make(map[uint16]uint32),
		}
	}

	s[hash] = spent
	return spent, nil
}

func (s SpentCoins) commit(b storage.Batch) error {
	buf := new(bytes.Buffer)
	for hash, state := range s {
		if err := state.EncodeBinary(buf); err != nil {
			return err
		}
		key := storage.AppendPrefix(storage.STSpentCoin, hash.BytesReverse())
		b.Put(key, buf.Bytes())
		buf.Reset()
	}
	return nil
}

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

// DecodeBinary implements the Payload interface.
func (s *SpentCoinState) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &s.txHash); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &s.txHeight); err != nil {
		return err
	}

	s.items = make(map[uint16]uint32)
	lenItems := util.ReadVarUint(r)
	for i := 0; i < int(lenItems); i++ {
		var (
			key   uint16
			value uint32
		)
		if err := binary.Read(r, binary.LittleEndian, &key); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &value); err != nil {
			return err
		}
		s.items[key] = value
	}
	return nil
}

// EncodeBinary implements the Payload interface.
func (s *SpentCoinState) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, s.txHash); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, s.txHeight); err != nil {
		return err
	}
	if err := util.WriteVarUint(w, uint64(len(s.items))); err != nil {
		return err
	}
	for k, v := range s.items {
		if err := binary.Write(w, binary.LittleEndian, k); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	return nil
}
