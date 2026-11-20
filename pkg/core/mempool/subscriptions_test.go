package mempool

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestSubscriptions(t *testing.T) {
	t.Run("disabled subscriptions", func(t *testing.T) {
		mp := New(5, false, nil)
		require.Panics(t, func() {
			mp.RunSubscriptions()
		})
		require.Panics(t, func() {
			mp.StopSubscriptions()
		})
	})

	t.Run("enabled subscriptions", func(t *testing.T) {
		fs := &FeerStub{balance: 100}
		mp := New(2, true, nil)
		mp.RunSubscriptions()
		subChan1 := make(chan mempoolevent.Event, 3)
		subChan2 := make(chan mempoolevent.Event, 3)
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
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionAdded, Tx: txs[0]}, event)

		// severak subscribers
		mp.SubscribeForTransactions(subChan2)
		require.NoError(t, mp.Add(txs[1], fs))
		require.Eventually(t, func() bool { return len(subChan1) == 1 && len(subChan2) == 1 }, time.Second, time.Millisecond*100)
		event1 := <-subChan1
		event2 := <-subChan2
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionAdded, Tx: txs[1]}, event1)
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionAdded, Tx: txs[1]}, event2)

		// reach capacity
		require.NoError(t, mp.Add(txs[2], &FeerStub{}))
		require.Eventually(t, func() bool { return len(subChan1) == 2 && len(subChan2) == 2 }, time.Second, time.Millisecond*100)
		event1 = <-subChan1
		event2 = <-subChan2
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionRemoved, Tx: txs[0]}, event1)
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionRemoved, Tx: txs[0]}, event2)
		event1 = <-subChan1
		event2 = <-subChan2
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionAdded, Tx: txs[2]}, event1)
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionAdded, Tx: txs[2]}, event2)

		// remove tx
		mp.Remove(txs[1].Hash())
		require.Eventually(t, func() bool { return len(subChan1) == 1 && len(subChan2) == 1 }, time.Second, time.Millisecond*100)
		event1 = <-subChan1
		event2 = <-subChan2
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionRemoved, Tx: txs[1]}, event1)
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionRemoved, Tx: txs[1]}, event2)

		// remove stale
		mp.RemoveStale(func(tx *transaction.Transaction) bool {
			return !tx.Hash().Equals(txs[2].Hash())
		}, fs)
		require.Eventually(t, func() bool { return len(subChan1) == 1 && len(subChan2) == 1 }, time.Second, time.Millisecond*100)
		event1 = <-subChan1
		event2 = <-subChan2
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionRemoved, Tx: txs[2]}, event1)
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionRemoved, Tx: txs[2]}, event2)

		// unsubscribe
		mp.UnsubscribeFromTransactions(subChan1)
		require.NoError(t, mp.Add(txs[3], fs))
		require.Eventually(t, func() bool { return len(subChan2) == 1 }, time.Second, time.Millisecond*100)
		event2 = <-subChan2
		require.Equal(t, 0, len(subChan1))
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionAdded, Tx: txs[3]}, event2)
	})

	t.Run("stop waits for in-flight event", func(t *testing.T) {
		fs := &FeerStub{balance: 100}
		mp := New(5, true, nil)

		// StopSubscriptions before RunSubscriptions must be a no-op.
		mp.StopSubscriptions()

		mp.RunSubscriptions()
		subCh := make(chan mempoolevent.Event)
		mp.SubscribeForTransactions(subCh)

		newTx := func(nonce uint32) *transaction.Transaction {
			tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
			tx.Nonce = nonce
			tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
			return tx
		}

		tx := newTx(0)
		require.NoError(t, mp.Add(tx, fs))

		stopDone := make(chan struct{})
		go func() {
			mp.StopSubscriptions()
			close(stopDone)
		}()

		require.Never(t, func() bool {
			select {
			case <-stopDone:
				return true
			default:
				return false
			}
		}, 100*time.Millisecond, 10*time.Millisecond)

		ev := <-subCh
		require.Equal(t, mempoolevent.Event{Type: mempoolevent.TransactionAdded, Tx: tx}, ev)

		select {
		case <-stopDone:
		case <-time.After(time.Second):
			require.FailNow(t, "StopSubscriptions did not return after draining in-flight event")
		}
		close(subCh)

		// Post-stop operations must not block.
		postDone := make(chan struct{})
		go func() {
			otherCh := make(chan mempoolevent.Event)
			mp.SubscribeForTransactions(otherCh)
			mp.UnsubscribeFromTransactions(otherCh)
			mp.StopSubscriptions()

			tx2 := newTx(1)
			require.NoError(t, mp.Add(tx2, fs))
			mp.Remove(tx2.Hash())

			tx3 := newTx(2)
			require.NoError(t, mp.Add(tx3, fs))
			mp.RemoveStale(func(*transaction.Transaction) bool { return false }, fs)
			require.Zero(t, mp.Count())
			close(postDone)
		}()
		select {
		case <-postDone:
		case <-time.After(time.Second):
			require.FailNow(t, "mempool operations blocked after StopSubscriptions")
		}
	})
}
