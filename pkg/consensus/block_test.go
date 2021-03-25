package consensus

import (
	"testing"

	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestNeoBlock_Sign(t *testing.T) {
	b := new(neoBlock)
	priv, _ := keys.NewPrivateKey()

	require.NoError(t, b.Sign(&privateKey{PrivateKey: priv}))
	require.NoError(t, b.Verify(&publicKey{PublicKey: priv.PublicKey()}, b.Signature()))
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

	b.Block.MerkleRoot = util.Uint256{1, 2, 3, 4}
	require.Equal(t, util.Uint256{1, 2, 3, 4}, b.MerkleRoot())

	b.Block.NextConsensus = util.Uint160{9, 2}
	require.Equal(t, util.Uint160{9, 2}, b.NextConsensus())

	b.Block.PrevHash = util.Uint256{9, 8, 7}
	require.Equal(t, util.Uint256{9, 8, 7}, b.PrevHash())

	txx := []block.Transaction{transaction.New([]byte{byte(opcode.PUSH1)}, 1)}
	b.SetTransactions(txx)
	require.Equal(t, txx, b.Transactions())
}
