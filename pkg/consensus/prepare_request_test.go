package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestPrepareRequest_Getters(t *testing.T) {
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	missing := map[util.Uint256]int{{1, 2, 3}: 1}
	p := &prepareRequest{
		version:   123,
		prevHash:  util.Uint256{1, 2, 3},
		timestamp: 123,
		verified:  []*transaction.Transaction{tx, nil},
		missing:   missing,
	}
	require.EqualValues(t, 123000000, p.Timestamp())
	txs, m := p.Transactions()
	require.Equal(t, []*transaction.Transaction{tx, nil}, txs)
	require.Equal(t, missing, m)
}

func TestPrepareRequest_EncodeDecodeBinary(t *testing.T) {
	checkBadTransactionsCount := func(t *testing.T, extended bool) {
		expected := &prepareRequest{timestamp: 112, extended: extended}
		if extended {
			txs := make([]*transaction.Transaction, block.MaxTransactionsPerBlock+1)
			for i := range txs {
				txs[i] = transaction.New([]byte{byte(opcode.PUSH1), byte(i), byte(i >> 8)}, 0)
			}
			expected.transactions = txs
		} else {
			expected.transactionHashes = make([]util.Uint256, block.MaxTransactionsPerBlock+1)
		}

		data, err := testserdes.EncodeBinary(expected)
		require.NoError(t, err)
		require.Error(t, testserdes.DecodeBinary(data, &prepareRequest{extended: extended}))
	}

	t.Run("extended", func(t *testing.T) {
		t.Run("positive", func(t *testing.T) {
			txs := []*transaction.Transaction{
				transaction.New([]byte{byte(opcode.PUSH1)}, 0),
				transaction.New([]byte{byte(opcode.PUSH2)}, 0),
			}
			addSender(t, txs...)
			for _, tx := range txs {
				tx.Scripts = []transaction.Witness{{InvocationScript: []byte{}, VerificationScript: []byte{}}}
				_ = tx.Hash() // Update hashes and serialized data.
				_ = tx.Size()
			}
			expected := &prepareRequest{
				timestamp:    112,
				extended:     true,
				transactions: txs,
			}
			testserdes.EncodeDecodeBinary(t, expected, &prepareRequest{extended: true})
		})

		t.Run("bad transactions count", func(t *testing.T) {
			checkBadTransactionsCount(t, true)
		})
	})

	t.Run("shortened", func(t *testing.T) {
		t.Run("positive", func(t *testing.T) {
			expected := &prepareRequest{
				timestamp:         112,
				transactionHashes: []util.Uint256{{1, 2, 3}, {4, 5, 6}},
			}
			testserdes.EncodeDecodeBinary(t, expected, new(prepareRequest))
		})

		t.Run("bad transactions count", func(t *testing.T) {
			checkBadTransactionsCount(t, false)
		})
	})
}
