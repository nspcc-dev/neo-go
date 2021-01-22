package state

import (
	"bytes"
	"errors"
	"math/big"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NFTTokenState represents state of nonfungible token.
type NFTTokenState struct {
	Owner       util.Uint160
	Name        string
	Description string
}

// NFTAccountState represents state of nonfunglible account.
type NFTAccountState struct {
	NEP17BalanceState
	Tokens [][]byte
}

// Base returns base class.
func (s *NFTTokenState) Base() *NFTTokenState {
	return s
}

// ToStackItem converts NFTTokenState to stackitem.
func (s *NFTTokenState) ToStackItem() stackitem.Item {
	owner := s.Owner
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray(owner.BytesBE()),
		stackitem.NewByteArray([]byte(s.Name)),
		stackitem.NewByteArray([]byte(s.Description)),
	})
}

// EncodeBinary implements io.Serializable.
func (s *NFTTokenState) EncodeBinary(w *io.BinWriter) {
	stackitem.EncodeBinaryStackItem(s.ToStackItem(), w)
}

// FromStackItem converts stackitem to NFTTokenState.
func (s *NFTTokenState) FromStackItem(item stackitem.Item) error {
	arr, ok := item.Value().([]stackitem.Item)
	if !ok || len(arr) < 3 {
		return errors.New("invalid stack item")
	}

	bs, err := arr[0].TryBytes()
	if err != nil {
		return err
	}
	owner, err := util.Uint160DecodeBytesBE(bs)
	if err != nil {
		return err
	}
	name, err := stackitem.ToString(arr[1])
	if err != nil {
		return err
	}
	desc, err := stackitem.ToString(arr[2])
	if err != nil {
		return err
	}

	s.Owner = owner
	s.Name = name
	s.Description = desc
	return nil
}

// DecodeBinary implements io.Serializable.
func (s *NFTTokenState) DecodeBinary(r *io.BinReader) {
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err == nil {
		r.Err = s.FromStackItem(item)
	}
}

// ToMap converts NFTTokenState to Map stackitem.
func (s *NFTTokenState) ToMap() *stackitem.Map {
	return stackitem.NewMapWithValue([]stackitem.MapElement{
		{
			Key:   stackitem.NewByteArray([]byte("name")),
			Value: stackitem.NewByteArray([]byte(s.Name)),
		},
		{
			Key:   stackitem.NewByteArray([]byte("description")),
			Value: stackitem.NewByteArray([]byte(s.Description)),
		},
	})
}

// ID returns token id.
func (s *NFTTokenState) ID() []byte {
	return []byte(s.Name)
}

// ToStackItem converts NFTAccountState to stackitem.
func (s *NFTAccountState) ToStackItem() stackitem.Item {
	st := s.NEP17BalanceState.toStackItem().(*stackitem.Struct)
	arr := make([]stackitem.Item, len(s.Tokens))
	for i := range arr {
		arr[i] = stackitem.NewByteArray(s.Tokens[i])
	}
	st.Append(stackitem.NewArray(arr))
	return st
}

// FromStackItem converts stackitem to NFTAccountState.
func (s *NFTAccountState) FromStackItem(item stackitem.Item) error {
	s.NEP17BalanceState.fromStackItem(item)
	arr := item.Value().([]stackitem.Item)
	if len(arr) < 2 {
		return errors.New("invalid stack item")
	}
	arr, ok := arr[1].Value().([]stackitem.Item)
	if !ok {
		return errors.New("invalid stack item")
	}
	s.Tokens = make([][]byte, len(arr))
	for i := range s.Tokens {
		bs, err := arr[i].TryBytes()
		if err != nil {
			return err
		}
		s.Tokens[i] = bs
	}
	return nil
}

// EncodeBinary implements io.Serializable.
func (s *NFTAccountState) EncodeBinary(w *io.BinWriter) {
	stackitem.EncodeBinaryStackItem(s.ToStackItem(), w)
}

// DecodeBinary implements io.Serializable.
func (s *NFTAccountState) DecodeBinary(r *io.BinReader) {
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err == nil {
		r.Err = s.FromStackItem(item)
	}
}

func (s *NFTAccountState) index(tokenID []byte) (int, bool) {
	lt := len(s.Tokens)
	index := sort.Search(lt, func(i int) bool {
		return bytes.Compare(s.Tokens[i], tokenID) >= 0
	})
	return index, index < lt && bytes.Equal(s.Tokens[index], tokenID)
}

// Add adds token id to the set of account tokens
// and returns true on success.
func (s *NFTAccountState) Add(tokenID []byte) bool {
	index, isPresent := s.index(tokenID)
	if isPresent {
		return false
	}

	s.Balance.Add(&s.Balance, big.NewInt(1))
	s.Tokens = append(s.Tokens, []byte{})
	copy(s.Tokens[index+1:], s.Tokens[index:])
	s.Tokens[index] = tokenID
	return true
}

// Remove removes token id to the set of account tokens
// and returns true on success.
func (s *NFTAccountState) Remove(tokenID []byte) bool {
	index, isPresent := s.index(tokenID)
	if !isPresent {
		return false
	}

	s.Balance.Sub(&s.Balance, big.NewInt(1))
	copy(s.Tokens[index:], s.Tokens[index+1:])
	s.Tokens = s.Tokens[:len(s.Tokens)-1]
	return true
}
