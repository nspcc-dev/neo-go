package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
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
	require.Equal(t, util.Uint160{5, 6, 7}, p.NextConsensus())

	p.SetNonce(8765)
	require.EqualValues(t, 8765, p.Nonce())

	hashes := [2]util.Uint256{random.Uint256(), random.Uint256()}

	p.SetTransactionHashes(hashes[:])
	require.Equal(t, hashes[:], p.TransactionHashes())
}
