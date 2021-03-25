package mempool

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestSubscriptions(t *testing.T) {
	t.Run("disabled subscriptions", func(t *testing.T) {
		mp := New(5, 0, false)
		require.Panics(t, func() {
			mp.RunSubscriptions()
		})
		require.Panics(t, func() {
			mp.StopSubscriptions()
		})
	})

	t.Run("enabled subscriptions", func(t *testing.T) {
		fs := &FeerStub{balance: 100}
		mp := New(2, 0, true)
		mp.RunSubscriptions()
		subChan1 := make(chan Event, 3)
		subChan2 := make(chan Event, 3)
		mp.SubscribeForTransactions(subChan1)
		t.Cleanup(mp.StopSubscriptions)

		txs := make([]*transaction.Transaction, 4)
		for i := range txs {
			txs[i] = transaction.New([]byte{byte(opcode.PUSH1)}, 0)
			txs[i].Nonce = uint32(i)
			txs[i].Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
			txs[i].NetworkFee = int64(i)
		}

		// add tx
		require.NoError(t, mp.Add(txs[0], fs))
		require.Eventually(t, func() bool { return len(subChan1) == 1 }, time.Second, time.Millisecond*100)
		event := <-subChan1
		require.Equal(t, Event{Type: TransactionAdded, Tx: txs[0]}, event)

		// severak subscribers
		mp.SubscribeForTransactions(subChan2)
		require.NoError(t, mp.Add(txs[1], fs))
		require.Eventually(t, func() bool { return len(subChan1) == 1 && len(subChan2) == 1 }, time.Second, time.Millisecond*100)
		event1 := <-subChan1
		event2 := <-subChan2
		require.Equal(t, Event{Type: TransactionAdded, Tx: txs[1]}, event1)
		require.Equal(t, Event{Type: TransactionAdded, Tx: txs[1]}, event2)

		// reach capacity
		require.NoError(t, mp.Add(txs[2], &FeerStub{}))
		require.Eventually(t, func() bool { return len(subChan1) == 2 && len(subChan2) == 2 }, time.Second, time.Millisecond*100)
		event1 = <-subChan1
		event2 = <-subChan2
		require.Equal(t, Event{Type: TransactionRemoved, Tx: txs[0]}, event1)
		require.Equal(t, Event{Type: TransactionRemoved, Tx: txs[0]}, event2)
		event1 = <-subChan1
		event2 = <-subChan2
		require.Equal(t, Event{Type: TransactionAdded, Tx: txs[2]}, event1)
		require.Equal(t, Event{Type: TransactionAdded, Tx: txs[2]}, event2)

		// remove tx
		mp.Remove(txs[1].Hash(), fs)
		require.Eventually(t, func() bool { return len(subChan1) == 1 && len(subChan2) == 1 }, time.Second, time.Millisecond*100)
		event1 = <-subChan1
		event2 = <-subChan2
		require.Equal(t, Event{Type: TransactionRemoved, Tx: txs[1]}, event1)
		require.Equal(t, Event{Type: TransactionRemoved, Tx: txs[1]}, event2)

		// remove stale
		mp.RemoveStale(func(tx *transaction.Transaction) bool {
			if tx.Hash().Equals(txs[2].Hash()) {
				return false
			}
			return true
		}, fs)
		require.Eventually(t, func() bool { return len(subChan1) == 1 && len(subChan2) == 1 }, time.Second, time.Millisecond*100)
		event1 = <-subChan1
		event2 = <-subChan2
		require.Equal(t, Event{Type: TransactionRemoved, Tx: txs[2]}, event1)
		require.Equal(t, Event{Type: TransactionRemoved, Tx: txs[2]}, event2)

		// unsubscribe
		mp.UnsubscribeFromTransactions(subChan1)
		require.NoError(t, mp.Add(txs[3], fs))
		require.Eventually(t, func() bool { return len(subChan2) == 1 }, time.Second, time.Millisecond*100)
		event2 = <-subChan2
		require.Equal(t, 0, len(subChan1))
		require.Equal(t, Event{Type: TransactionAdded, Tx: txs[3]}, event2)
	})
}
