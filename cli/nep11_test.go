package main

import (
	"os"
	"path"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/stretchr/testify/require"
)

func TestNEP11Import(t *testing.T) {
	e := newExecutor(t, true)

	tmpDir := os.TempDir()
	walletPath := path.Join(tmpDir, "walletForImport.json")
	defer os.Remove(walletPath)

	nnsContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.NameService)
	require.NoError(t, err)
	neoContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.Neo)
	require.NoError(t, err)
	e.Run(t, "neo-go", "wallet", "init", "--wallet", walletPath)

	args := []string{
		"neo-go", "wallet", "nep11", "import",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", walletPath,
	}
	// missing token hash
	e.RunWithError(t, args...)

	// good
	e.Run(t, append(args, "--token", nnsContractHash.StringLE())...)

	// already exists
	e.RunWithError(t, append(args, "--token", nnsContractHash.StringLE())...)

	// not a NEP11 token
	e.RunWithError(t, append(args, "--token", neoContractHash.StringLE())...)
}
