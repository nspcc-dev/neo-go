package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestPrepareRequest_Setters(t *testing.T) {
	var p prepareRequest

	p.SetTimestamp(123)
	// 123ns -> 0ms -> 0ns
	require.EqualValues(t, 0, p.Timestamp())

	p.SetTimestamp(1230000)
	// 1230000ns -> 1ms -> 1000000ns
	require.EqualValues(t, 1000000, p.Timestamp())

	p.SetNextConsensus(util.Uint160{5, 6, 7})
	require.Equal(t, util.Uint160{}, p.NextConsensus())

	hashes := [2]util.Uint256{random.Uint256(), random.Uint256()}

	p.SetTransactionHashes(hashes[:])
	require.Equal(t, hashes[:], p.TransactionHashes())
}

func TestPrepareRequest_EncodeDecodeBinary(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		expected := &prepareRequest{
			timestamp: 112,
			transactionHashes: []util.Uint256{
				random.Uint256(),
				random.Uint256(),
			},
		}
		testserdes.EncodeDecodeBinary(t, expected, new(prepareRequest))
	})

	t.Run("bad hashes count", func(t *testing.T) {
		hashes := make([]util.Uint256, block.MaxTransactionsPerBlock+1)
		for i := range hashes {
			hashes[i] = random.Uint256()
		}
		expected := &prepareRequest{
			timestamp:         112,
			transactionHashes: hashes,
		}
		data, err := testserdes.EncodeBinary(expected)
		require.NoError(t, err)
		require.Error(t, testserdes.DecodeBinary(data, new(prepareRequest)))
	})
}
