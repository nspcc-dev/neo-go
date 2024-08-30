package mempool

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	poolSize = 10000
)

func BenchmarkPool(b *testing.B) {
	fe := &FeerStub{
		feePerByte:  1,
		blockHeight: 1,
		balance:     100_0000_0000,
	}
	txesSimple := make([]*transaction.Transaction, poolSize)
	for i := range txesSimple {
		txesSimple[i] = transaction.New([]byte{1, 2, 3}, 100500)
		txesSimple[i].Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	}
	txesIncFee := make([]*transaction.Transaction, poolSize)
	for i := range txesIncFee {
		txesIncFee[i] = transaction.New([]byte{1, 2, 3}, 100500)
		txesIncFee[i].NetworkFee = 10 * int64(i)
		txesIncFee[i].Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	}
	txesMulti := make([]*transaction.Transaction, poolSize)
	for i := range txesMulti {
		txesMulti[i] = transaction.New([]byte{1, 2, 3}, 100500)
		txesMulti[i].Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3, byte(i % 256), byte(i / 256)}}}
	}
	txesMultiInc := make([]*transaction.Transaction, poolSize)
	for i := range txesMultiInc {
		txesMultiInc[i] = transaction.New([]byte{1, 2, 3}, 100500)
		txesMultiInc[i].NetworkFee = 10 * int64(i)
		txesMultiInc[i].Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3, byte(i % 256), byte(i / 256)}}}
	}

	senders := make(map[string][]*transaction.Transaction)
	senders["one, same fee"] = txesSimple
	senders["one, incr fee"] = txesIncFee
	senders["many, same fee"] = txesMulti
	senders["many, incr fee"] = txesMultiInc
	for name, txes := range senders {
		b.Run(name, func(b *testing.B) {
			p := New(poolSize, 0, false, nil)
			b.ResetTimer()
			for range b.N {
				for j := range txes {
					if p.Add(txes[j], fe) != nil {
						b.Fail()
					}
				}
				p.RemoveStale(func(*transaction.Transaction) bool { return false }, fe)
			}
		})
	}
}
