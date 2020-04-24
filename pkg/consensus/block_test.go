package consensus

import (
	"crypto/rand"
	"testing"

	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestNeoBlock_Sign(t *testing.T) {
	b := new(neoBlock)
	priv, pub := crypto.Generate(rand.Reader)

	require.NoError(t, b.Sign(priv))
	require.NoError(t, b.Verify(pub, b.Signature()))
}

func TestNeoBlock_Setters(t *testing.T) {
	b := new(neoBlock)

	b.SetVersion(1)
	require.EqualValues(t, 1, b.Version())

	b.SetIndex(12)
	require.EqualValues(t, 12, b.Index())

	b.SetTimestamp(777)
	// 777ns -> 0ms -> 0ns
	require.EqualValues(t, 0, b.Timestamp())

	b.SetTimestamp(7777777)
	// 7777777ns -> 7ms -> 7000000ns
	require.EqualValues(t, 7000000, b.Timestamp())

	b.SetConsensusData(456)
	require.EqualValues(t, 456, b.ConsensusData())

	b.SetMerkleRoot(util.Uint256{1, 2, 3, 4})
	require.Equal(t, util.Uint256{1, 2, 3, 4}, b.MerkleRoot())

	b.SetNextConsensus(util.Uint160{9, 2})
	require.Equal(t, util.Uint160{9, 2}, b.NextConsensus())

	b.SetPrevHash(util.Uint256{9, 8, 7})
	require.Equal(t, util.Uint256{9, 8, 7}, b.PrevHash())

	txx := []block.Transaction{transaction.NewMinerTX()}
	b.SetTransactions(txx)
	require.Equal(t, txx, b.Transactions())
}
