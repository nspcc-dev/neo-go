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
	var p = &prepareRequest{
		version:   123,
		prevHash:  util.Uint256{1, 2, 3},
		timestamp: 123,
		transactions: []*transaction.Transaction{
			transaction.New([]byte{byte(opcode.PUSH1)}, 0),
		},
	}

	require.EqualValues(t, 123000000, p.Timestamp())
	// TransactionHashes is intentionally unused and always empty, see Transactions instead.
	require.Empty(t, p.TransactionHashes())
}

func TestPrepareRequest_EncodeDecodeBinary(t *testing.T) {
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
			transactions: txs,
		}
		testserdes.EncodeDecodeBinary(t, expected, new(prepareRequest))
	})

	t.Run("bad transactions count", func(t *testing.T) {
		txs := make([]*transaction.Transaction, block.MaxTransactionsPerBlock+1)
		for i := range txs {
			txs[i] = transaction.New([]byte{byte(opcode.PUSH1), byte(i), byte(i >> 8)}, 0)
		}
		expected := &prepareRequest{
			timestamp:    112,
			transactions: txs,
		}
		data, err := testserdes.EncodeBinary(expected)
		require.NoError(t, err)
		require.Error(t, testserdes.DecodeBinary(data, new(prepareRequest)))
	})
}
