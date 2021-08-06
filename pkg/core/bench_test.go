package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func BenchmarkVerifyWitness(t *testing.B) {
	bc := newTestChain(t)
	acc, err := wallet.NewAccount()
	require.NoError(t, err)

	tx := bc.newTestTx(acc.Contract.ScriptHash(), []byte{byte(opcode.PUSH1)})
	require.NoError(t, acc.SignTx(netmode.UnitTestNet, tx))

	t.ResetTimer()
	for n := 0; n < t.N; n++ {
		_ = bc.VerifyWitness(tx.Signers[0].Account, tx, &tx.Scripts[0], 100000000)
	}
}
