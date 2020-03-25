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
