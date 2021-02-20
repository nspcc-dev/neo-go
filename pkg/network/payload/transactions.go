package payload

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Transactions represents batch of transactions.
type Transactions struct {
	Values []*transaction.Transaction
}

// MaxBatchSize is maximum amount of transactions in batch.
const MaxBatchSize = MaxHashesCount

// DecodeBinary implements io.Serializable interface.
func (t *Transactions) DecodeBinary(r *io.BinReader) {
	l := r.ReadVarUint()
	if l == 0 {
		r.Err = errors.New("empty batch")
		return
	}
	if l > MaxBatchSize {
		r.Err = errors.New("batch is too big")
		return
	}

	t.Values = make([]*transaction.Transaction, l)
	for i := uint64(0); i < l; i++ {
		tx := new(transaction.Transaction)
		tx.DecodeBinary(r)
		if r.Err != nil {
			return
		}
		t.Values[i] = tx
	}
}

// EncodeBinary implements io.Serializable interface.
func (t *Transactions) EncodeBinary(w *io.BinWriter) {
	w.WriteVarUint(uint64(len(t.Values)))
	for i := range t.Values {
		t.Values[i].EncodeBinary(w)
	}
}
