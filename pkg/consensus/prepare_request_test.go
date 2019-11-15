package consensus

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestPrepareRequest_Setters(t *testing.T) {
	var p prepareRequest

	p.SetTimestamp(123)
	require.EqualValues(t, 123, p.Timestamp())

	p.SetNextConsensus(util.Uint160{5, 6, 7})
	require.Equal(t, util.Uint160{5, 6, 7}, p.NextConsensus())

	p.SetNonce(8765)
	require.EqualValues(t, 8765, p.Nonce())

	var hashes [2]util.Uint256
	fillRandom(t, hashes[0][:])
	fillRandom(t, hashes[1][:])

	p.SetTransactionHashes(hashes[:])
	require.Equal(t, hashes[:], p.TransactionHashes())
}
