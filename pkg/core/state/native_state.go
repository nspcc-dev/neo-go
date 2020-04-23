package state

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// NEP5BalanceState represents balance state of a NEP5-token.
type NEP5BalanceState struct {
	Balance big.Int
}

// NEOBalanceState represents balance state of a NEO-token.
type NEOBalanceState struct {
	NEP5BalanceState
	BalanceHeight uint32
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

// EncodeBinary implements io.Serializable interface.
func (s *NEP5BalanceState) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(emit.IntToBytes(&s.Balance))
}

// DecodeBinary implements io.Serializable interface.
func (s *NEP5BalanceState) DecodeBinary(r *io.BinReader) {
	buf := r.ReadVarBytes()
	if r.Err != nil {
		return
	}
	s.Balance = *emit.BytesToInt(buf)
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
	s.NEP5BalanceState.EncodeBinary(w)
	w.WriteU32LE(s.BalanceHeight)
}

// DecodeBinary implements io.Serializable interface.
func (s *NEOBalanceState) DecodeBinary(r *io.BinReader) {
	s.NEP5BalanceState.DecodeBinary(r)
	s.BalanceHeight = r.ReadU32LE()
}
