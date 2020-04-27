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

	b.Block.Version = 1
	require.EqualValues(t, 1, b.Version())

	b.Block.Index = 12
	require.EqualValues(t, 12, b.Index())

	b.Block.Timestamp = 777
	// 777ms -> 777000000ns
	require.EqualValues(t, 777000000, b.Timestamp())

	b.Block.ConsensusData.Nonce = 456
	require.EqualValues(t, 456, b.ConsensusData())

	b.Block.MerkleRoot = util.Uint256{1, 2, 3, 4}
	require.Equal(t, util.Uint256{1, 2, 3, 4}, b.MerkleRoot())

	b.Block.NextConsensus = util.Uint160{9, 2}
	require.Equal(t, util.Uint160{9, 2}, b.NextConsensus())

	b.Block.PrevHash = util.Uint256{9, 8, 7}
	require.Equal(t, util.Uint256{9, 8, 7}, b.PrevHash())

	txx := []block.Transaction{transaction.NewIssueTX()}
	b.SetTransactions(txx)
	require.Equal(t, txx, b.Transactions())
}
