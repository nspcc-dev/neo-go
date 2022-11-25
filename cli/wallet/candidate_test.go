package wallet_test

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testcli"
	"github.com/stretchr/testify/require"
)

// Register standby validator and vote for it.
// We don't create a new account here, because chain will
// stop working after validator will change.
func TestRegisterCandidate(t *testing.T) {
	e := testcli.NewExecutor(t, true)

	validatorAddress := testcli.ValidatorPriv.Address()
	validatorPublic := testcli.ValidatorPriv.PublicKey()
	validatorHex := hex.EncodeToString(validatorPublic.Bytes())

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "nep17", "multitransfer",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--from", testcli.ValidatorAddr,
		"--force",
		"NEO:"+validatorAddress+":10",
		"GAS:"+validatorAddress+":10000")
	e.CheckTxPersisted(t)

	e.Run(t, "neo-go", "query", "committee",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0])
	e.CheckNextLine(t, "^\\s*"+validatorHex)

	e.Run(t, "neo-go", "query", "candidates",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0])
	e.CheckNextLine(t, "^\\s*Key.+$") // Header.
	e.CheckEOF(t)

	// missing address
	e.RunWithError(t, "neo-go", "wallet", "candidate", "register",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet)

	// additional parameter
	e.RunWithError(t, "neo-go", "wallet", "candidate", "register",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--address", validatorAddress,
		"error")

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "candidate", "register",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--address", validatorAddress,
		"--force")
	e.CheckTxPersisted(t)

	vs, err := e.Chain.GetEnrollments()
	require.NoError(t, err)
	require.Equal(t, 1, len(vs))
	require.Equal(t, validatorPublic, vs[0].Key)
	require.Equal(t, big.NewInt(0), vs[0].Votes)

	t.Run("VoteUnvote", func(t *testing.T) {
		// positional instead of a flag.
		e.RunWithError(t, "neo-go", "wallet", "candidate", "vote",
			"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
			"--wallet", testcli.ValidatorWallet,
			"--address", validatorAddress,
			validatorHex) // not "--candidate hex", but "hex".

		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "wallet", "candidate", "vote",
			"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
			"--wallet", testcli.ValidatorWallet,
			"--address", validatorAddress,
			"--candidate", validatorHex,
			"--force")
		_, index := e.CheckTxPersisted(t)

		vs, err = e.Chain.GetEnrollments()
		require.Equal(t, 1, len(vs))
		require.Equal(t, validatorPublic, vs[0].Key)
		b, _ := e.Chain.GetGoverningTokenBalance(testcli.ValidatorPriv.GetScriptHash())
		require.Equal(t, b, vs[0].Votes)

		e.Run(t, "neo-go", "query", "committee",
			"--rpc-endpoint", "http://"+e.RPC.Addresses()[0])
		e.CheckNextLine(t, "^\\s*"+validatorHex)

		e.Run(t, "neo-go", "query", "candidates",
			"--rpc-endpoint", "http://"+e.RPC.Addresses()[0])
		e.CheckNextLine(t, "^\\s*Key.+$") // Header.
		e.CheckNextLine(t, "^\\s*"+validatorHex+"\\s*"+b.String()+"\\s*true\\s*true$")
		e.CheckEOF(t)

		// check state
		e.Run(t, "neo-go", "query", "voter",
			"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
			validatorAddress)
		e.CheckNextLine(t, "^\\s*Voted:\\s+"+validatorHex+"\\s+\\("+validatorAddress+"\\)$")
		e.CheckNextLine(t, "^\\s*Amount\\s*:\\s*"+b.String()+"$")
		e.CheckNextLine(t, "^\\s*Block\\s*:\\s*"+strconv.FormatUint(uint64(index), 10))
		e.CheckEOF(t)

		// unvote
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "wallet", "candidate", "vote",
			"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
			"--wallet", testcli.ValidatorWallet,
			"--address", validatorAddress,
			"--force")
		_, index = e.CheckTxPersisted(t)

		vs, err = e.Chain.GetEnrollments()
		require.Equal(t, 1, len(vs))
		require.Equal(t, validatorPublic, vs[0].Key)
		require.Equal(t, big.NewInt(0), vs[0].Votes)

		// check state
		e.Run(t, "neo-go", "query", "voter",
			"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
			validatorAddress)
		e.CheckNextLine(t, "^\\s*Voted:\\s+"+"null") // no vote.
		e.CheckNextLine(t, "^\\s*Amount\\s*:\\s*"+b.String()+"$")
		e.CheckNextLine(t, "^\\s*Block\\s*:\\s*"+strconv.FormatUint(uint64(index), 10))
		e.CheckEOF(t)
	})

	// missing address
	e.RunWithError(t, "neo-go", "wallet", "candidate", "unregister",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet)
	// additional argument
	e.RunWithError(t, "neo-go", "wallet", "candidate", "unregister",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--address", validatorAddress,
		"argument")

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "candidate", "unregister",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
		"--wallet", testcli.ValidatorWallet,
		"--address", validatorAddress,
		"--force")
	e.CheckTxPersisted(t)

	vs, err = e.Chain.GetEnrollments()
	require.Equal(t, 0, len(vs))

	// query voter: missing address
	e.RunWithError(t, "neo-go", "query", "voter")
	// Excessive parameters.
	e.RunWithError(t, "neo-go", "query", "voter", "--rpc-endpoint", "http://"+e.RPC.Addresses()[0], validatorAddress, validatorAddress)
	e.RunWithError(t, "neo-go", "query", "committee", "--rpc-endpoint", "http://"+e.RPC.Addresses()[0], "something")
	e.RunWithError(t, "neo-go", "query", "candidates", "--rpc-endpoint", "http://"+e.RPC.Addresses()[0], "something")
}
