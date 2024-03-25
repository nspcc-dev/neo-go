package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestPrepareRequest_Getters(t *testing.T) {
	hashes := []util.Uint256{random.Uint256(), random.Uint256()}
	var p = &prepareRequest{
		version:           123,
		prevHash:          util.Uint256{1, 2, 3},
		timestamp:         123,
		transactionHashes: hashes,
	}

	require.EqualValues(t, 123000000, p.Timestamp())
	require.Equal(t, hashes, p.TransactionHashes())
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
