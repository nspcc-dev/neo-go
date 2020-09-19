package main

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

// Register standby validator and vote for it.
// We don't create a new account here, because chain will
// stop working after validator will change.
func TestRegisterCandidate(t *testing.T) {
	e := newExecutor(t, true)
	defer e.Close(t)

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "nep5", "multitransfer",
		"--unittest", "--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--from", validatorAddr,
		"neo:"+validatorPriv.Address()+":10",
		"gas:"+validatorPriv.Address()+":100")
	e.checkTxPersisted(t)

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "candidate", "register",
		"--unittest", "--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--address", validatorPriv.Address())
	e.checkTxPersisted(t)

	vs, err := e.Chain.GetEnrollments()
	require.NoError(t, err)
	require.Equal(t, 1, len(vs))
	require.Equal(t, validatorPriv.PublicKey(), vs[0].Key)
	require.Equal(t, big.NewInt(0), vs[0].Votes)

	t.Run("Vote", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "wallet", "candidate", "vote",
			"--unittest", "--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet,
			"--address", validatorPriv.Address(),
			"--candidate", hex.EncodeToString(validatorPriv.PublicKey().Bytes()))
		e.checkTxPersisted(t)

		vs, err = e.Chain.GetEnrollments()
		require.Equal(t, 1, len(vs))
		require.Equal(t, validatorPriv.PublicKey(), vs[0].Key)
		b, _ := e.Chain.GetGoverningTokenBalance(validatorPriv.GetScriptHash())
		require.Equal(t, b, vs[0].Votes)
	})

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "candidate", "unregister",
		"--unittest", "--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--address", validatorPriv.Address())
	e.checkTxPersisted(t)

	vs, err = e.Chain.GetEnrollments()
	require.Equal(t, 0, len(vs))
}
