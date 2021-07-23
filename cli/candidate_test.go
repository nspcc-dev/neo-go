package main

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// Register standby validator and vote for it.
// We don't create a new account here, because chain will
// stop working after validator will change.
func TestRegisterCandidate(t *testing.T) {
	e := newExecutor(t, true)

	validatorHex := hex.EncodeToString(validatorPriv.PublicKey().Bytes())

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "nep17", "multitransfer",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--from", validatorAddr,
		"NEO:"+validatorPriv.Address()+":10",
		"GAS:"+validatorPriv.Address()+":10000")
	e.checkTxPersisted(t)

	e.Run(t, "neo-go", "query", "committee",
		"--rpc-endpoint", "http://"+e.RPC.Addr)
	e.checkNextLine(t, "^\\s*"+validatorHex)

	e.Run(t, "neo-go", "query", "candidates",
		"--rpc-endpoint", "http://"+e.RPC.Addr)
	e.checkNextLine(t, "^\\s*Key.+$") // Header.
	e.checkEOF(t)

	// missing address
	e.RunWithError(t, "neo-go", "wallet", "candidate", "register",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet)

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "candidate", "register",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--address", validatorPriv.Address())
	e.checkTxPersisted(t)

	vs, err := e.Chain.GetEnrollments()
	require.NoError(t, err)
	require.Equal(t, 1, len(vs))
	require.Equal(t, validatorPriv.PublicKey(), vs[0].Key)
	require.Equal(t, big.NewInt(0), vs[0].Votes)

	t.Run("VoteUnvote", func(t *testing.T) {
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "wallet", "candidate", "vote",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet,
			"--address", validatorPriv.Address(),
			"--candidate", validatorHex)
		_, index := e.checkTxPersisted(t)

		vs, err = e.Chain.GetEnrollments()
		require.Equal(t, 1, len(vs))
		require.Equal(t, validatorPriv.PublicKey(), vs[0].Key)
		b, _ := e.Chain.GetGoverningTokenBalance(validatorPriv.GetScriptHash())
		require.Equal(t, b, vs[0].Votes)

		e.Run(t, "neo-go", "query", "committee",
			"--rpc-endpoint", "http://"+e.RPC.Addr)
		e.checkNextLine(t, "^\\s*"+validatorHex)

		e.Run(t, "neo-go", "query", "candidates",
			"--rpc-endpoint", "http://"+e.RPC.Addr)
		e.checkNextLine(t, "^\\s*Key.+$") // Header.
		e.checkNextLine(t, "^\\s*"+validatorHex+"\\s*"+b.String()+"\\s*true\\s*true$")
		e.checkEOF(t)

		// check state
		e.Run(t, "neo-go", "query", "voter",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			validatorPriv.Address())
		e.checkNextLine(t, "^\\s*Voted:\\s+"+validatorHex+"\\s+\\("+validatorPriv.Address()+"\\)$")
		e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+b.String()+"$")
		e.checkNextLine(t, "^\\s*Block\\s*:\\s*"+strconv.FormatUint(uint64(index), 10))
		e.checkEOF(t)

		// unvote
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "wallet", "candidate", "vote",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet,
			"--address", validatorPriv.Address())
		_, index = e.checkTxPersisted(t)

		vs, err = e.Chain.GetEnrollments()
		require.Equal(t, 1, len(vs))
		require.Equal(t, validatorPriv.PublicKey(), vs[0].Key)
		require.Equal(t, big.NewInt(0), vs[0].Votes)

		// check state
		e.Run(t, "neo-go", "query", "voter",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			validatorPriv.Address())
		e.checkNextLine(t, "^\\s*Voted:\\s+"+"null") // no vote.
		e.checkNextLine(t, "^\\s*Amount\\s*:\\s*"+b.String()+"$")
		e.checkNextLine(t, "^\\s*Block\\s*:\\s*"+strconv.FormatUint(uint64(index), 10))
		e.checkEOF(t)
	})

	// missing address
	e.RunWithError(t, "neo-go", "wallet", "candidate", "unregister",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet)

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "candidate", "unregister",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--address", validatorPriv.Address())
	e.checkTxPersisted(t)

	vs, err = e.Chain.GetEnrollments()
	require.Equal(t, 0, len(vs))

	// query voter: missing address
	e.RunWithError(t, "neo-go", "query", "voter")
}
