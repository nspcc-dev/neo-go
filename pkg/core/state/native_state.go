package state

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NEP17BalanceState represents balance state of a NEP17-token.
type NEP17BalanceState struct {
	Balance big.Int
}

// NEOBalanceState represents balance state of a NEO-token.
type NEOBalanceState struct {
	NEP17BalanceState
	BalanceHeight uint32
	VoteTo        *keys.PublicKey
}

// NEP17BalanceStateFromBytes converts serialized NEP17BalanceState to structure.
func NEP17BalanceStateFromBytes(b []byte) (*NEP17BalanceState, error) {
	balance := new(NEP17BalanceState)
	if len(b) == 0 {
		return balance, nil
	}
	r := io.NewBinReaderFromBuf(b)
	balance.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return balance, nil
}

// Bytes returns serialized NEP17BalanceState.
func (s *NEP17BalanceState) Bytes() []byte {
	w := io.NewBufBinWriter()
	s.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		panic(w.Err)
	}
	return w.Bytes()
}

func (s *NEP17BalanceState) toStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{stackitem.NewBigInteger(&s.Balance)})
}

func (s *NEP17BalanceState) fromStackItem(item stackitem.Item) {
	s.Balance = *item.(*stackitem.Struct).Value().([]stackitem.Item)[0].Value().(*big.Int)
}

// EncodeBinary implements io.Serializable interface.
func (s *NEP17BalanceState) EncodeBinary(w *io.BinWriter) {
	si := s.toStackItem()
	stackitem.EncodeBinaryStackItem(si, w)
}

// DecodeBinary implements io.Serializable interface.
func (s *NEP17BalanceState) DecodeBinary(r *io.BinReader) {
	si := stackitem.DecodeBinaryStackItem(r)
	if r.Err != nil {
		return
	}
	s.fromStackItem(si)
}

// NEOBalanceStateFromBytes converts serialized NEOBalanceState to structure.
func NEOBalanceStateFromBytes(b []byte) (*NEOBalanceState, error) {
	balance := new(NEOBalanceState)
	if len(b) == 0 {
		return balance, nil
	}
	r := io.NewBinReaderFromBuf(b)
	balance.DecodeBinary(r)

	if r.Err != nil {
		return nil, r.Err
	}
	return balance, nil
}

// Bytes returns serialized NEOBalanceState.
func (s *NEOBalanceState) Bytes() []byte {
	w := io.NewBufBinWriter()
	s.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		panic(w.Err)
	}
	return w.Bytes()
}

// EncodeBinary implements io.Serializable interface.
func (s *NEOBalanceState) EncodeBinary(w *io.BinWriter) {
	si := s.toStackItem()
	stackitem.EncodeBinaryStackItem(si, w)
}

// DecodeBinary implements io.Serializable interface.
func (s *NEOBalanceState) DecodeBinary(r *io.BinReader) {
	si := stackitem.DecodeBinaryStackItem(r)
	if r.Err != nil {
		return
	}
	r.Err = s.FromStackItem(si)
}

func (s *NEOBalanceState) toStackItem() stackitem.Item {
	result := s.NEP17BalanceState.toStackItem().(*stackitem.Struct)
	result.Append(stackitem.NewBigInteger(big.NewInt(int64(s.BalanceHeight))))
	if s.VoteTo != nil {
		result.Append(stackitem.NewByteArray(s.VoteTo.Bytes()))
	} else {
		result.Append(stackitem.Null{})
	}
	return result
}

// FromStackItem converts stackitem.Item to NEOBalanceState.
func (s *NEOBalanceState) FromStackItem(item stackitem.Item) error {
	structItem, ok := item.Value().([]stackitem.Item)
	if !ok || len(structItem) < 3 {
		return errors.New("invalid stackitem length")
	}
	balance, err := structItem[0].TryInteger()
	if err != nil {
		return fmt.Errorf("invalid balance stackitem: %w", err)
	}
	s.Balance = *balance
	h, err := structItem[1].TryInteger()
	if err != nil {
		return fmt.Errorf("invalid heigh stackitem")
	}
	s.BalanceHeight = uint32(h.Int64())
	if _, ok := structItem[2].(stackitem.Null); ok {
		s.VoteTo = nil
		return nil
	}
	bs, err := structItem[2].TryBytes()
	if err != nil {
		return fmt.Errorf("invalid public key stackitem: %w", err)
	}
	pub, err := keys.NewPublicKeyFromBytes(bs, elliptic.P256())
	if err != nil {
		return fmt.Errorf("invalid public key bytes: %w", err)
	}
	s.VoteTo = pub
	return nil
}
