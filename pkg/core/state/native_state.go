package state

import (
	"crypto/elliptic"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NEP5BalanceState represents balance state of a NEP5-token.
type NEP5BalanceState struct {
	Balance big.Int
}

// NEOBalanceState represents balance state of a NEO-token.
type NEOBalanceState struct {
	NEP5BalanceState
	BalanceHeight uint32
	VoteTo        *keys.PublicKey
}

// NEP5BalanceStateFromBytes converts serialized NEP5BalanceState to structure.
func NEP5BalanceStateFromBytes(b []byte) (*NEP5BalanceState, error) {
	balance := new(NEP5BalanceState)
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

// Bytes returns serialized NEP5BalanceState.
func (s *NEP5BalanceState) Bytes() []byte {
	w := io.NewBufBinWriter()
	s.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		panic(w.Err)
	}
	return w.Bytes()
}

func (s *NEP5BalanceState) toStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{stackitem.NewBigInteger(&s.Balance)})
}

func (s *NEP5BalanceState) fromStackItem(item stackitem.Item) {
	s.Balance = *item.(*stackitem.Struct).Value().([]stackitem.Item)[0].Value().(*big.Int)
}

// EncodeBinary implements io.Serializable interface.
func (s *NEP5BalanceState) EncodeBinary(w *io.BinWriter) {
	si := s.toStackItem()
	stackitem.EncodeBinaryStackItem(si, w)
}

// DecodeBinary implements io.Serializable interface.
func (s *NEP5BalanceState) DecodeBinary(r *io.BinReader) {
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
	r.Err = s.fromStackItem(si)
}

func (s *NEOBalanceState) toStackItem() stackitem.Item {
	result := s.NEP5BalanceState.toStackItem().(*stackitem.Struct)
	result.Append(stackitem.NewBigInteger(big.NewInt(int64(s.BalanceHeight))))
	if s.VoteTo != nil {
		result.Append(stackitem.NewByteArray(s.VoteTo.Bytes()))
	} else {
		result.Append(stackitem.Null{})
	}
	return result
}

func (s *NEOBalanceState) fromStackItem(item stackitem.Item) error {
	structItem := item.Value().([]stackitem.Item)
	s.Balance = *structItem[0].Value().(*big.Int)
	s.BalanceHeight = uint32(structItem[1].Value().(*big.Int).Int64())
	if _, ok := structItem[2].(stackitem.Null); ok {
		s.VoteTo = nil
		return nil
	}
	bs, err := structItem[2].TryBytes()
	if err != nil {
		return err
	}
	pub, err := keys.NewPublicKeyFromBytes(bs, elliptic.P256())
	if err != nil {
		return err
	}
	s.VoteTo = pub
	return nil
}
