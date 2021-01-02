package state

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// UnspentCoin hold the state of a unspent coin.
type UnspentCoin struct {
	Height uint32
	States []OutputState
}

// OutputState combines transaction output (UTXO) and its state
// (spent/claimed...) along with the height of spend (if it's spent).
type OutputState struct {
	transaction.Output

	SpendHeight uint32
	State       Coin
}

// NewUnspentCoin returns a new unspent coin state with N confirmed states.
func NewUnspentCoin(height uint32, tx *transaction.Transaction) *UnspentCoin {
	u := &UnspentCoin{
		Height: height,
		States: make([]OutputState, len(tx.Outputs)),
	}
	for i := range tx.Outputs {
		u.States[i] = OutputState{Output: tx.Outputs[i]}
	}
	return u
}

// EncodeBinary encodes UnspentCoin to the given BinWriter.
func (s *UnspentCoin) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(s.Height)
	bw.WriteArray(s.States)
	bw.WriteVarUint(uint64(len(s.States)))
}

// DecodeBinary decodes UnspentCoin from the given BinReader.
func (s *UnspentCoin) DecodeBinary(br *io.BinReader) {
	s.Height = br.ReadU32LE()
	br.ReadArray(&s.States)
	if br.Err == nil {
		for i := range s.States {
			s.States[i].Output.Position = i
		}
	}
}

// EncodeBinary implements Serializable interface.
func (o *OutputState) EncodeBinary(w *io.BinWriter) {
	o.Output.EncodeBinary(w)
	w.WriteU32LE(o.SpendHeight)
	w.WriteB(byte(o.State))
}

// DecodeBinary implements Serializable interface.
func (o *OutputState) DecodeBinary(r *io.BinReader) {
	o.Output.DecodeBinary(r)
	o.SpendHeight = r.ReadU32LE()
	o.State = Coin(r.ReadB())
}
